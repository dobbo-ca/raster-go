package raster_test

import (
	"context"
	"errors"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"

	raster "github.com/dobbo-ca/raster-go"
	"github.com/dobbo-ca/raster-go/internal/fixture"
)

// bitmap places a w*h packed bitmap at the end of a guard-page mapping.
func bitmap(t testing.TB, seed uint64, w, h, pad int) raster.Bitmap {
	stride := (w+7)/8 + pad
	buf, free := fixture.Guarded((h-1)*stride + (w+7)/8)
	t.Cleanup(free)
	copy(buf, fixture.Bytes(seed, len(buf)))
	return raster.Bitmap{Pix: buf, W: w, H: h, Stride: stride}
}

func bit(b raster.Bitmap, x, y int) int { return int(b.Pix[y*b.Stride+x/8]>>(7-x%8)) & 1 }

// checkSubsample compares against the pure-Go expectation: the source pixel
// at the centre of each destination pixel, (2d+1)*s/(2*dstN), MSB first, pad
// bits zero.
func checkSubsample(t *testing.T, dst, src raster.Bitmap) {
	t.Helper()
	for y := 0; y < dst.H; y++ {
		sy := (2*y + 1) * src.H / (2 * dst.H)
		for x := 0; x < dst.W; x++ {
			if g, w := bit(dst, x, y), bit(src, (2*x+1)*src.W/(2*dst.W), sy); g != w {
				t.Fatalf("%dx%d->%dx%d (%d,%d) = %d, want %d", src.W, src.H, dst.W, dst.H, x, y, g, w)
			}
		}
		for x := dst.W; x < (dst.W+7)/8*8; x++ {
			if bit(dst, x, y) != 0 {
				t.Fatalf("pad bit set at (%d,%d)", x, y)
			}
		}
	}
}

func TestSubsample1bpp(t *testing.T) {
	for _, sw := range []int{1, 15, 16, 17, 31, 32, 33, 63, 64, 65} {
		for _, dw := range []int{1, 7, 8, 9, 16, 17, 33} {
			src := bitmap(t, uint64(sw), sw, 21, 1)
			dst := bitmap(t, 9, dw, 5, 3)
			if err := raster.Subsample1bpp(bg, dst, src, raster.Limits{}); err != nil {
				t.Fatal(err)
			}
			checkSubsample(t, dst, src)
		}
	}
}

func TestSubsample1bppRefusesBeforeAllocating(t *testing.T) {
	// real buffers, refused on the scratch cap: nothing may be allocated first
	src := bitmap(t, 1, 4000, 4000, 0)
	dst := bitmap(t, 2, 2000, 2000, 0)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := raster.Subsample1bpp(bg, dst, src, raster.Limits{MaxScratchBytes: 7999})
	runtime.ReadMemStats(&after)
	var le *raster.LimitError
	if !errors.As(err, &le) || le.Name != "scratch bytes" {
		t.Fatalf("got %v", err)
	}
	if d := after.TotalAlloc - before.TotalAlloc; d > 1<<16 {
		t.Fatalf("allocated %d bytes before refusing", d)
	}
	// huge dims: refused as invalid or over limit, never allocated or run
	huge := raster.Bitmap{Pix: make([]byte, 1), W: 1 << 30, H: 1 << 30, Stride: 1 << 27}
	for _, e := range []error{
		raster.Subsample1bpp(bg, dst, huge, raster.Limits{}),
		raster.Subsample1bpp(bg, huge, src, raster.Limits{}),
	} {
		if !errors.Is(e, raster.ErrInvalid) && !errors.Is(e, raster.ErrLimit) {
			t.Fatalf("huge dims: %v", e)
		}
	}
	over := bitmap(t, 3, 64, 64, 0)
	if err := raster.Subsample1bpp(bg, over, over, raster.Limits{MaxPixels: 4095}); !errors.Is(err, raster.ErrLimit) {
		t.Fatalf("pixels: %v", err)
	}
	if err := raster.Subsample1bpp(bg, dst, src, raster.Limits{MaxOutputBytes: 1}); !errors.Is(err, raster.ErrLimit) {
		t.Fatalf("output: %v", err)
	}
	if err := raster.Subsample1bpp(bg, dst, src, raster.Limits{MaxPixels: -1}); !errors.Is(err, raster.ErrInvalid) {
		t.Fatalf("negative: %v", err)
	}
	short := raster.Bitmap{Pix: make([]byte, 1), W: 16, H: 2, Stride: 2}
	if err := raster.Subsample1bpp(bg, dst, short, raster.Limits{}); !errors.Is(err, raster.ErrInvalid) {
		t.Fatalf("short: %v", err)
	}
}

// The x-index table is int32, so a source wider than MaxInt32 is refused even
// when the caller raised MaxPixels.
func TestSubsample1bppWideSourceRefused(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("needs 64-bit int")
	}
	w := math.MaxInt32 + 1
	src := raster.Bitmap{Pix: make([]byte, (w+7)/8), W: w, H: 1, Stride: (w + 7) / 8}
	dst := bitmap(t, 1, 8, 1, 0)
	err := raster.Subsample1bpp(context.Background(), dst, src, raster.Limits{MaxPixels: 1 << 40})
	if !errors.Is(err, raster.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestSubsample1bppHugeDimsRefused(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("needs 64-bit int")
	}
	big := math.MaxInt32 + 1
	lim := raster.Limits{MaxPixels: 1 << 40, MaxOutputBytes: 1 << 40, MaxScratchBytes: 1 << 40}
	for name, c := range map[string][2]raster.Bitmap{
		"src.H": {{W: 1, H: 1, Stride: 1}, {W: 1, H: big, Stride: 1}},
		"dst.H": {{W: 1, H: big, Stride: 1}, {W: 1, H: 1, Stride: 1}},
		"dst.W": {{W: 1, H: 1, Stride: 1}, {W: big, H: 1, Stride: (big + 7) / 8}},
	} {
		if err := raster.Subsample1bpp(context.Background(), c[0], c[1], lim); !errors.Is(err, raster.ErrInvalid) || !strings.Contains(err.Error(), "MaxInt32") {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestSubsample1bppCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if err := raster.Subsample1bpp(ctx, bitmap(t, 1, 8, 8, 0), bitmap(t, 2, 16, 16, 0), raster.Limits{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func FuzzSubsample1bpp(f *testing.F) {
	f.Add(uint8(70), uint8(40), uint8(30), uint8(20), uint8(1), uint64(1))
	f.Fuzz(func(t *testing.T, w, h, dw, dh, pad uint8, seed uint64) {
		src := bitmap(t, seed, int(w)%90+1, int(h)%90+1, int(pad)%5)
		dst := bitmap(t, seed+1, int(dw)%120+1, int(dh)%120+1, int(pad)%3)
		must(t, raster.Subsample1bpp(bg, dst, src, raster.Limits{}))
		checkSubsample(t, dst, src)
	})
}
