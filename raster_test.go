package raster_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	raster "github.com/dobbo-ca/raster-go"
	"github.com/dobbo-ca/raster-go/internal/cpu"
	"github.com/dobbo-ca/raster-go/internal/fixture"
	"github.com/dobbo-ca/raster-go/internal/resample"
)

var bg = context.Background()

var widths = []int{1, 2, 3, 15, 16, 17, 18, 19, 31, 32, 33, 34, 35, 63, 64, 65, 66, 67, 127, 128, 129, 130, 255, 257}

// plane places a w*h plane of ch channels at the very end of a guard-page
// mapping, so an overread by asm faults.
func plane(t testing.TB, seed uint64, w, h, ch, pad int) raster.Plane {
	stride := w*ch + pad
	buf, free := fixture.Guarded((h-1)*stride + w*ch)
	t.Cleanup(free)
	fill := fixture.Page(seed, w, h, ch)
	for y := 0; y < h; y++ {
		copy(buf[y*stride:], fill[y*w*ch:(y+1)*w*ch])
	}
	return raster.Plane{Pix: buf, W: w, H: h, Stride: stride, Channels: ch}
}

func blank(t testing.TB, w, h, ch, pad int) raster.Plane {
	p := plane(t, 0, w, h, ch, pad)
	clear(p.Pix)
	return p
}

func at(p raster.Plane, x, y, c int) int { return int(p.Pix[y*p.Stride+x*p.Channels+c]) }

func same(t *testing.T, name string, got raster.Plane, want func(x, y, c int) int) {
	t.Helper()
	for y := 0; y < got.H; y++ {
		for x := 0; x < got.W; x++ {
			for c := 0; c < got.Channels; c++ {
				if g, w := at(got, x, y, c), want(x, y, c); g != w {
					t.Fatalf("%s: (%d,%d,%d) = %d, want %d", name, x, y, c, g, w)
				}
			}
		}
	}
}

func TestScale2to1(t *testing.T) {
	for _, ch := range []int{1, 3} {
		for _, w := range widths {
			for _, pad := range []int{0, 3} {
				src := plane(t, uint64(w), w, 5, ch, pad)
				dst := blank(t, w/2, 2, ch, pad)
				err := raster.Scale2to1(bg, dst, src, raster.Limits{})
				if w < 2 {
					if !errors.Is(err, raster.ErrInvalid) {
						t.Fatalf("w=%d: err %v, want ErrInvalid", w, err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				same(t, fmt.Sprintf("ch=%d w=%d", ch, w), dst, func(x, y, c int) int {
					return (at(src, 2*x, 2*y, c) + at(src, 2*x+1, 2*y, c) + at(src, 2*x, 2*y+1, c) + at(src, 2*x+1, 2*y+1, c) + 2) >> 2
				})
			}
		}
	}
}

func lumaOf(r, g, b int) int { return (38*r + 75*g + 15*b + 64) >> 7 }

func TestLuma(t *testing.T) {
	for _, w := range widths {
		for _, pad := range []int{0, 3} {
			src := plane(t, uint64(w), w, 3, 3, pad)
			dst := blank(t, w, 3, 1, pad)
			if err := raster.Luma(bg, dst, src, raster.Limits{}); err != nil {
				t.Fatal(err)
			}
			same(t, fmt.Sprintf("w=%d", w), dst, func(x, y, _ int) int {
				return lumaOf(at(src, x, y, 0), at(src, x, y, 1), at(src, x, y, 2))
			})
		}
	}
}

func TestLumaExtremes(t *testing.T) {
	for _, v := range []byte{0, 255} {
		src := blank(t, 40, 1, 3, 0)
		for i := range src.Pix {
			src.Pix[i] = v
		}
		dst := blank(t, 40, 1, 1, 0)
		if err := raster.Luma(bg, dst, src, raster.Limits{}); err != nil {
			t.Fatal(err)
		}
		for _, g := range dst.Pix {
			if g != v {
				t.Fatalf("grey of %d = %d", v, g)
			}
		}
	}
}

func TestRotate90(t *testing.T) {
	for _, ch := range []int{1, 3} {
		for _, wh := range [][2]int{{1, 1}, {1, 7}, {7, 1}, {16, 16}, {17, 33}, {40, 5}} {
			w, h := wh[0], wh[1]
			src := plane(t, 9, w, h, ch, 3)
			for _, cw := range []bool{true, false} {
				dst := blank(t, h, w, ch, 1)
				if err := raster.Rotate90(bg, dst, src, cw, raster.Limits{}); err != nil {
					t.Fatal(err)
				}
				same(t, fmt.Sprintf("cw=%v %dx%d", cw, w, h), dst, func(x, y, c int) int {
					if cw {
						return at(src, y, h-1-x, c)
					}
					return at(src, w-1-y, x, c)
				})
			}
		}
	}
}

func TestResample(t *testing.T) {
	for _, ch := range []int{1, 3} {
		// identity is exact
		src := plane(t, 5, 37, 29, ch, 2)
		dst := blank(t, 37, 29, ch, 0)
		if err := raster.Resample(bg, dst, src, raster.Limits{}); err != nil {
			t.Fatal(err)
		}
		same(t, "identity", dst, func(x, y, c int) int { return at(src, x, y, c) })
		// a flat plane stays flat at any ratio, up or down
		for _, v := range []byte{0, 77, 255} {
			flat := blank(t, 64, 48, ch, 0)
			for i := range flat.Pix {
				flat.Pix[i] = v
			}
			for _, d := range [][2]int{{32, 24}, {5, 3}, {1, 1}, {100, 90}, {64, 1}} {
				out := blank(t, d[0], d[1], ch, 0)
				if err := raster.Resample(bg, out, flat, raster.Limits{}); err != nil {
					t.Fatal(err)
				}
				same(t, fmt.Sprintf("flat %d -> %v", v, d), out, func(_, _, _ int) int { return int(v) })
			}
		}
	}
	// a horizontal ramp halves to a ramp, within a level
	ramp := blank(t, 256, 4, 1, 0)
	for y := 0; y < 4; y++ {
		for x := 0; x < 256; x++ {
			ramp.Pix[y*256+x] = byte(x)
		}
	}
	out := blank(t, 128, 2, 1, 0)
	if err := raster.Resample(bg, out, ramp, raster.Limits{}); err != nil {
		t.Fatal(err)
	}
	for x := 4; x < 124; x++ {
		if want := 2*x + 1; math.Abs(float64(at(out, x, 0, 0)-want)) > 1 {
			t.Fatalf("ramp x=%d: %d, want %d+-1", x, at(out, x, 0, 0), want)
		}
	}
}

func TestInvalid(t *testing.T) {
	ok := blank(t, 8, 8, 1, 0)
	half := blank(t, 4, 4, 1, 0)
	bad := []raster.Plane{
		{},
		{Pix: make([]byte, 64), W: 8, H: 8, Stride: 7, Channels: 1},
		{Pix: make([]byte, 63), W: 8, H: 8, Stride: 8, Channels: 1},
		{Pix: make([]byte, 64), W: 8, H: 8, Stride: 8, Channels: 2},
		{Pix: make([]byte, 64), W: math.MaxInt, H: 2, Stride: math.MaxInt, Channels: 1},
		{Pix: make([]byte, 64), W: 8, H: math.MaxInt, Stride: 8, Channels: 1},
		{Pix: make([]byte, 64), W: 1, H: 2, Stride: math.MaxInt, Channels: 1},
	}
	for i, p := range bad {
		for name, err := range map[string]error{
			"src":    raster.Scale2to1(bg, half, p, raster.Limits{}),
			"dst":    raster.Scale2to1(bg, p, ok, raster.Limits{}),
			"resamp": raster.Resample(bg, p, ok, raster.Limits{}),
		} {
			if !errors.Is(err, raster.ErrInvalid) {
				t.Fatalf("bad plane %d as %s: %v", i, name, err)
			}
		}
	}
	if err := raster.Scale2to1(bg, ok, ok, raster.Limits{}); !errors.Is(err, raster.ErrInvalid) {
		t.Fatalf("wrong dst size: %v", err)
	}
	if err := raster.Scale2to1(bg, half, ok, raster.Limits{MaxPixels: -1}); !errors.Is(err, raster.ErrInvalid) {
		t.Fatalf("negative limit: %v", err)
	}
}

func TestLimits(t *testing.T) {
	src := plane(t, 1, 64, 64, 1, 0)
	dst := blank(t, 32, 32, 1, 0)
	for i := range dst.Pix {
		dst.Pix[i] = 0xAA
	}
	for _, tc := range []struct {
		name string
		lim  raster.Limits
	}{
		{"pixels", raster.Limits{MaxPixels: 4095}},
		{"output bytes", raster.Limits{MaxOutputBytes: 1023}},
	} {
		err := raster.Scale2to1(bg, dst, src, tc.lim)
		var le *raster.LimitError
		if !errors.Is(err, raster.ErrLimit) || !errors.As(err, &le) || le.Name != tc.name || le.Got <= le.Max {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if dst.Pix[0] != 0xAA {
			t.Fatalf("%s: dst written despite breach", tc.name)
		}
	}
	// the exact cap passes
	if err := raster.Scale2to1(bg, dst, src, raster.Limits{MaxPixels: 4096, MaxOutputBytes: 1024}); err != nil {
		t.Fatal(err)
	}
	// the default pixel cap refuses a plane over 1<<26 before touching it
	big := raster.Plane{Pix: make([]byte, 1), W: 1 << 14, H: 1<<12 + 1, Stride: 1 << 14, Channels: 1}
	if err := raster.Luma(bg, dst, big, raster.Limits{}); !errors.Is(err, raster.ErrInvalid) {
		t.Fatalf("short buffer must fail validation first: %v", err)
	}
}

// A Resample over the scratch cap must refuse before it allocates.
func TestResampleScratchLimit(t *testing.T) {
	src := plane(t, 1, 2000, 2000, 1, 0)
	dst := blank(t, 1000, 1000, 1, 0)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := raster.Resample(bg, dst, src, raster.Limits{MaxScratchBytes: 1 << 20})
	runtime.ReadMemStats(&after)
	var le *raster.LimitError
	if !errors.As(err, &le) || le.Name != "scratch bytes" {
		t.Fatalf("got %v", err)
	}
	if d := after.TotalAlloc - before.TotalAlloc; d > 1<<16 {
		t.Fatalf("allocated %d bytes before refusing", d)
	}
	if err := raster.Resample(bg, dst, src, raster.Limits{MaxScratchBytes: le.Got}); err != nil {
		t.Fatalf("cap at the reported need must pass: %v", err)
	}
}

// An allowed Resample must not allocate more than the need it reported.
func TestResampleScratchExact(t *testing.T) {
	for _, c := range [][4]int{{200000, 4, 1, 1}, {4, 200000, 1, 1}, {3000, 2000, 700, 500}} {
		src := plane(t, 1, c[0], c[1], 1, 0)
		dst := blank(t, c[2], c[3], 1, 0)
		need := int64(resample.ScratchBytes(c[0], c[1], c[2], c[3], 1))
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		err := raster.Resample(bg, dst, src, raster.Limits{MaxScratchBytes: need})
		runtime.ReadMemStats(&after)
		must(t, err)
		if d := int64(after.TotalAlloc - before.TotalAlloc); d > need+1<<16 { // slack: runtime noise, as in TestResampleScratchLimit
			t.Errorf("%v: allocated %d, reported need %d", c, d, need)
		}
	}
}

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(bg)
	cancel()
	g := plane(t, 1, 64, 64, 1, 0)
	c := plane(t, 1, 64, 64, 3, 0)
	h := blank(t, 32, 32, 1, 0)
	for name, err := range map[string]error{
		"scale":    raster.Scale2to1(ctx, h, g, raster.Limits{}),
		"luma":     raster.Luma(ctx, blank(t, 64, 64, 1, 0), c, raster.Limits{}),
		"rotate":   raster.Rotate90(ctx, blank(t, 64, 64, 1, 0), g, true, raster.Limits{}),
		"resample": raster.Resample(ctx, h, g, raster.Limits{}),
	} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

type goldenCase struct {
	name string
	run  func(t *testing.T) []byte
}

func goldenCases() []goldenCase {
	var cs []goldenCase
	for _, sz := range [][2]int{{257, 131}, {640, 480}} {
		w, h := sz[0], sz[1]
		for _, ch := range []int{1, 3} {
			cs = append(cs, goldenCase{fmt.Sprintf("scale2to1/%dx%dx%d", w, h, ch), func(t *testing.T) []byte {
				d := blank(t, w/2, h/2, ch, 0)
				must(t, raster.Scale2to1(bg, d, plane(t, 1, w, h, ch, 0), raster.Limits{}))
				return d.Pix
			}})
			for _, d := range [][2]int{{w / 2, h / 2}, {101, 77}, {w + 60, h + 20}, {w, 1}} {
				cs = append(cs, goldenCase{fmt.Sprintf("resample/%dx%dx%d->%dx%d", w, h, ch, d[0], d[1]), func(t *testing.T) []byte {
					o := blank(t, d[0], d[1], ch, 0)
					must(t, raster.Resample(bg, o, plane(t, 2, w, h, ch, 0), raster.Limits{}))
					return o.Pix
				}})
			}
			for _, cw := range []bool{true, false} {
				cs = append(cs, goldenCase{fmt.Sprintf("rotate90/%dx%dx%d/cw=%v", w, h, ch, cw), func(t *testing.T) []byte {
					o := blank(t, h, w, ch, 0)
					must(t, raster.Rotate90(bg, o, plane(t, 3, w, h, ch, 0), cw, raster.Limits{}))
					return o.Pix
				}})
			}
		}
		cs = append(cs, goldenCase{fmt.Sprintf("luma/%dx%d", w, h), func(t *testing.T) []byte {
			o := blank(t, w, h, 1, 0)
			must(t, raster.Luma(bg, o, plane(t, 4, w, h, 3, 0), raster.Limits{}))
			return o.Pix
		}})
	}
	return cs
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The goldens were generated on amd64 with -tags purego GOAMD64=v1
// (RASTERGO_UPDATE_GOLDEN=1); every tier on every arch must reproduce them.
func TestGolden(t *testing.T) {
	const file = "testdata/golden/hashes.txt"
	var out bytes.Buffer
	got := map[string]string{}
	for _, c := range goldenCases() {
		sum := fmt.Sprintf("%x", sha256.Sum256(c.run(t)))
		got[c.name] = sum
		fmt.Fprintf(&out, "%s %s\n", c.name, sum)
	}
	if os.Getenv("RASTERGO_UPDATE_GOLDEN") != "" {
		must(t, os.WriteFile(file, out.Bytes(), 0o644))
		return
	}
	data, err := os.ReadFile(file)
	must(t, err)
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		name, sum, _ := strings.Cut(line, " ")
		if got[name] != sum {
			t.Errorf("%s: %s, golden %s", name, got[name], sum)
		}
		n++
	}
	if n != len(got) {
		t.Fatalf("golden has %d lines, %d cases", n, len(got))
	}
}

func TestImpl(t *testing.T) {
	data, err := os.ReadFile("testdata/impl_expected.txt")
	must(t, err)
	seen := 0
	for _, line := range strings.Split(string(data), "\n") {
		fs := strings.Fields(line)
		if len(fs) < 3 || fs[0] == "#" || fs[1] != runtime.GOARCH {
			continue
		}
		seen++
		want := "generic" // purego, a forced generic tier or a host without the ISA
		if !purego && !slices.Contains([]string{"generic"}, cpu.Forced) {
			for _, tier := range fs[2:] {
				if tier == "generic" || (tier == "avx2" && cpu.AVX2()) || (tier == "neon" && cpu.NEON()) {
					want = tier
					break
				}
			}
		}
		// CI exports the ISA it read from /proc/cpuinfo, so a detector that
		// always says no fails here instead of agreeing with itself.
		if e := os.Getenv("RASTERGO_EXPECT"); e != "" && !purego && cpu.Forced != "generic" && slices.Contains(fs[2:], e) {
			want = e
		}
		if got := raster.Impl(fs[0]); got != want {
			t.Errorf("%s: Impl %q, want %q (table %v, RASTERGO_CPU=%q)", fs[0], got, want, fs[2:], cpu.Forced)
		}
	}
	if seen == 0 {
		t.Skipf("no table lines for %s", runtime.GOARCH)
	}
	t.Logf("RASTERGO_CPU=%q avx2=%v neon=%v", cpu.Forced, cpu.AVX2(), cpu.NEON())
}

func FuzzScale2to1(f *testing.F) {
	f.Add(uint8(70), uint8(5), uint8(1), uint8(0), uint64(1))
	f.Fuzz(func(t *testing.T, w, h, ch, pad uint8, seed uint64) {
		ch = 1 + 2*(ch&1)
		wi, hi := int(w)%130+2, int(h)%6+2
		src := plane(t, seed, wi, hi, int(ch), int(pad)%5)
		dst := blank(t, wi/2, hi/2, int(ch), int(pad)%3)
		must(t, raster.Scale2to1(bg, dst, src, raster.Limits{}))
		same(t, "fuzz", dst, func(x, y, c int) int {
			return (at(src, 2*x, 2*y, c) + at(src, 2*x+1, 2*y, c) + at(src, 2*x, 2*y+1, c) + at(src, 2*x+1, 2*y+1, c) + 2) >> 2
		})
	})
}

func FuzzLuma(f *testing.F) {
	f.Add(uint8(70), uint8(3), uint8(0), uint64(1))
	f.Fuzz(func(t *testing.T, w, h, pad uint8, seed uint64) {
		wi, hi := int(w)%130+1, int(h)%6+1
		src := plane(t, seed, wi, hi, 3, int(pad)%5)
		dst := blank(t, wi, hi, 1, int(pad)%3)
		must(t, raster.Luma(bg, dst, src, raster.Limits{}))
		same(t, "fuzz", dst, func(x, y, _ int) int { return lumaOf(at(src, x, y, 0), at(src, x, y, 1), at(src, x, y, 2)) })
	})
}

func FuzzRotate90(f *testing.F) {
	f.Add(uint8(20), uint8(33), uint8(1), true, uint64(1))
	f.Fuzz(func(t *testing.T, w, h, ch uint8, cw bool, seed uint64) {
		ch = 1 + 2*(ch&1)
		wi, hi := int(w)%50+1, int(h)%50+1
		src := plane(t, seed, wi, hi, int(ch), 1)
		dst := blank(t, hi, wi, int(ch), 0)
		must(t, raster.Rotate90(bg, dst, src, cw, raster.Limits{}))
		same(t, "fuzz", dst, func(x, y, c int) int {
			if cw {
				return at(src, y, hi-1-x, c)
			}
			return at(src, wi-1-y, x, c)
		})
	})
}

func FuzzResample(f *testing.F) {
	f.Add(uint8(60), uint8(40), uint8(30), uint8(20), uint8(1), uint64(1))
	f.Fuzz(func(t *testing.T, w, h, dw, dh, ch uint8, seed uint64) {
		ch = 1 + 2*(ch&1)
		src := plane(t, seed, int(w)%90+1, int(h)%90+1, int(ch), 2)
		dst := blank(t, int(dw)%120+1, int(dh)%120+1, int(ch), 0)
		must(t, raster.Resample(bg, dst, src, raster.Limits{}))
	})
}
