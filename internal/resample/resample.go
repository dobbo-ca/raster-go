// Package resample is the separable Catmull-Rom scaler: Q14 int16 taps, int32
// accumulation, an 8-bit intermediate plane. Go only; see docs/decisions.md.
package resample

import (
	"context"
	"math"
)

const one = 1 << 14 // Q14 unity

type taps struct {
	start  []int32 // first source index per output
	cnt    []int32 // tap count per output
	w      []int16 // out*stride taps, zero padded
	stride int
}

func fscale(srcN, dstN int) float64 { return math.Max(float64(srcN)/float64(dstN), 1) }

// tapStride is the widest tap row; a row never has more taps than source samples.
func tapStride(srcN, dstN int) int64 {
	return min(int64(math.Ceil(float64(4*fscale(srcN, dstN))))+2, int64(srcN))
}

func mulSat(a, b int64) int64 {
	if a != 0 && b > math.MaxInt64/a {
		return math.MaxInt64
	}
	return a * b
}

func addSat(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// ScratchBytes is what Run allocates: the intermediate plane, both tap tables,
// the float64 weight row build uses per axis and one int32 accumulator row. It saturates instead of overflowing.
func ScratchBytes(sw, sh, dw, dh, ch int) int64 {
	n := mulSat(mulSat(int64(sh), int64(dw)), int64(ch))
	n = addSat(n, mulSat(int64(dw), 2*tapStride(sw, dw)+8))
	n = addSat(n, mulSat(int64(dh), 2*tapStride(sh, dh)+8))
	n = addSat(n, 8*(tapStride(sw, dw)+tapStride(sh, dh)))
	return addSat(n, mulSat(mulSat(int64(dw), int64(ch)), 4))
}

// cubic is the Catmull-Rom kernel (a = -0.5). Every product is wrapped in
// float64() so arm64 cannot fuse it: the taps are bit-identical across arches.
func cubic(x float64) float64 {
	x = math.Abs(x)
	x2 := float64(x * x)
	x3 := float64(x2 * x)
	switch {
	case x < 1:
		return float64(1.5*x3) - float64(2.5*x2) + 1
	case x < 2:
		return float64(-0.5*x3) + float64(2.5*x2) - float64(4*x) + 2
	}
	return 0
}

func build(srcN, dstN int) taps {
	st := int(tapStride(srcN, dstN))
	t := taps{make([]int32, dstN), make([]int32, dstN), make([]int16, dstN*st), st}
	s := float64(srcN) / float64(dstN)
	f := fscale(srcN, dstN)
	sup := float64(2 * f)
	fw := make([]float64, st)
	for i := 0; i < dstN; i++ {
		c := float64((float64(i)+0.5)*s) - 0.5
		lo := max(int(math.Ceil(c-sup)), 0)
		hi := min(int(math.Floor(c+sup)), srcN-1)
		if hi < lo { // sliver at an edge: nearest sample
			lo = min(max(int(math.Floor(c+0.5)), 0), srcN-1)
			hi = lo
		}
		n := hi - lo + 1
		sum := 0.0
		for k := 0; k < n; k++ {
			fw[k] = cubic((float64(lo+k) - c) / f)
			sum += fw[k]
		}
		if sum == 0 { // all taps on a zero crossing: nearest sample
			fw[0], sum, n, lo = 1, 1, 1, min(max(int(math.Floor(c+0.5)), 0), srcN-1)
		}
		row := t.w[i*st:][:n]
		tot, big := 0, 0
		for k := 0; k < n; k++ {
			q := int(math.Floor(float64(fw[k]/sum*one) + 0.5))
			row[k] = int16(q)
			tot += q
			if row[k] > row[big] {
				big = k
			}
		}
		row[big] += int16(one - tot) // taps sum to exactly 1.0
		t.start[i], t.cnt[i] = int32(lo), int32(n)
	}
	return t
}

func clamp8(acc int32) byte {
	switch v := acc >> 14; {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return byte(v)
	}
}

// Run resamples; the caller has validated planes and bounded ScratchBytes.
func Run(ctx context.Context, dst []byte, dstStride, dw, dh int, src []byte, srcStride, sw, sh, ch int) error {
	hx, vy := build(sw, dw), build(sh, dh)
	mrow := dw * ch
	mid := make([]byte, sh*mrow)
	for y := 0; y < sh; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		in, out := src[y*srcStride:][:sw*ch], mid[y*mrow:][:mrow]
		for x := 0; x < dw; x++ {
			ws := hx.w[x*hx.stride:][:hx.cnt[x]]
			base := int(hx.start[x]) * ch
			for c := 0; c < ch; c++ {
				acc := int32(one / 2)
				for k, w := range ws {
					acc += int32(w) * int32(in[base+k*ch+c])
				}
				out[x*ch+c] = clamp8(acc)
			}
		}
	}
	acc := make([]int32, mrow)
	for y := 0; y < dh; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for i := range acc {
			acc[i] = one / 2
		}
		ws := vy.w[y*vy.stride:][:vy.cnt[y]]
		for k, w := range ws {
			row := mid[(int(vy.start[y])+k)*mrow:][:mrow]
			w32 := int32(w)
			for i, v := range row {
				acc[i] += w32 * int32(v)
			}
		}
		out := dst[y*dstStride:][:mrow]
		for i, a := range acc {
			out[i] = clamp8(a)
		}
	}
	return nil
}
