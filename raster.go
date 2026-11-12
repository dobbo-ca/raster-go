package raster

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/dobbo-ca/raster-go/internal/luma"
	"github.com/dobbo-ca/raster-go/internal/resample"
	"github.com/dobbo-ca/raster-go/internal/rotate"
	"github.com/dobbo-ca/raster-go/internal/scale"
	"github.com/dobbo-ca/raster-go/internal/subsample"
)

// ErrInvalid wraps every argument error: a bad plane, mismatched dimensions
// or a negative limit. No asm runs on invalid input.
var ErrInvalid = errors.New("raster: invalid argument")

// ErrLimit wraps every *LimitError (permanent: retrying with the same input
// and Limits fails again).
var ErrLimit = errors.New("raster: limit exceeded")

// LimitError reports which cap a call breached, before it allocated or wrote.
type LimitError struct {
	Name     string // "pixels", "scratch bytes" or "output bytes"
	Got, Max int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("raster: %s %d exceeds limit %d", e.Name, e.Got, e.Max)
}

func (e *LimitError) Unwrap() error { return ErrLimit }

// Default caps used for a zero Limits field.
const (
	DefaultMaxPixels       = 1 << 26
	DefaultMaxScratchBytes = 1 << 28
	DefaultMaxOutputBytes  = 1 << 28
)

// Limits caps one call. The zero value of each field selects the default;
// negative values are invalid. Raster-go has no package budget: the caller
// reserves MaxScratchBytes from its own budget.
type Limits struct {
	MaxPixels       int64 // pixels per call: the larger of source and destination
	MaxScratchBytes int64 // bytes the call may allocate internally
	MaxOutputBytes  int64 // bytes written to the destination
}

// Plane is an 8-bit interleaved raster: Channels is 1 (grey) or 3 (RGB), and
// row y starts at Pix[y*Stride].
type Plane struct {
	Pix                    []byte
	W, H, Stride, Channels int
}

// Bitmap is a packed 1-bpp raster, most significant bit first: pixel x of row
// y is bit 7-x%8 of Pix[y*Stride+x/8]. Stride is in bytes.
type Bitmap struct {
	Pix          []byte
	W, H, Stride int
}

func (b Bitmap) validate(name string) error {
	if b.W <= 0 || b.H <= 0 {
		return fmt.Errorf("%w: %s: bad width or height", ErrInvalid, name)
	}
	if b.W > math.MaxInt-7 {
		return fmt.Errorf("%w: %s: width overflows", ErrInvalid, name)
	}
	row := (b.W + 7) / 8
	if b.Stride < row {
		return fmt.Errorf("%w: %s: stride below row bytes", ErrInvalid, name)
	}
	if b.H-1 > (math.MaxInt-row)/b.Stride {
		return fmt.Errorf("%w: %s: size overflows", ErrInvalid, name)
	}
	if len(b.Pix) < (b.H-1)*b.Stride+row {
		return fmt.Errorf("%w: %s: buffer too short", ErrInvalid, name)
	}
	return nil
}

func (p Plane) validate(name string) error {
	if p.W <= 0 || p.H <= 0 || (p.Channels != 1 && p.Channels != 3) {
		return fmt.Errorf("%w: %s: bad width, height or channels", ErrInvalid, name)
	}
	if p.W > math.MaxInt/p.Channels {
		return fmt.Errorf("%w: %s: width overflows", ErrInvalid, name)
	}
	row := p.W * p.Channels
	if p.Stride < row {
		return fmt.Errorf("%w: %s: stride below row bytes", ErrInvalid, name)
	}
	if p.H-1 > (math.MaxInt-row)/p.Stride {
		return fmt.Errorf("%w: %s: size overflows", ErrInvalid, name)
	}
	if len(p.Pix) < (p.H-1)*p.Stride+row {
		return fmt.Errorf("%w: %s: buffer too short", ErrInvalid, name)
	}
	return nil
}

func (p Plane) pixels() int64 { return int64(p.W) * int64(p.H) }

func (p Plane) bytes() int64 { return p.pixels() * int64(p.Channels) }

// check validates both planes and the limits; scratch is the bytes the call
// will allocate.
func check(dst, src Plane, scratch int64, lim Limits) error {
	if err := src.validate("src"); err != nil {
		return err
	}
	if err := dst.validate("dst"); err != nil {
		return err
	}
	return checkLimits(src.pixels(), dst.pixels(), dst.bytes(), scratch, lim)
}

// checkLimits applies lim to already-validated sizes.
func checkLimits(srcPixels, dstPixels, dstBytes, scratch int64, lim Limits) error {
	if lim.MaxPixels < 0 || lim.MaxScratchBytes < 0 || lim.MaxOutputBytes < 0 {
		return fmt.Errorf("%w: negative limit", ErrInvalid)
	}
	for _, c := range [...]struct {
		name     string
		got, max int64
		def      int64
	}{
		{"pixels", max(srcPixels, dstPixels), lim.MaxPixels, DefaultMaxPixels},
		{"scratch bytes", scratch, lim.MaxScratchBytes, DefaultMaxScratchBytes},
		{"output bytes", dstBytes, lim.MaxOutputBytes, DefaultMaxOutputBytes},
	} {
		m := c.max
		if m == 0 {
			m = c.def
		}
		if c.got > m {
			return &LimitError{Name: c.name, Got: c.got, Max: m}
		}
	}
	return nil
}

// Scale2to1 halves src exactly: each output sample is the rounded mean
// (sum+2)>>2 of a 2x2 block. dst must be src.W/2 by src.H/2 with the same
// Channels; an odd last row or column is dropped. ctx is checked between rows.
// dst must not overlap src.
func Scale2to1(ctx context.Context, dst, src Plane, lim Limits) error {
	if err := check(dst, src, 0, lim); err != nil {
		return err
	}
	if src.W < 2 || src.H < 2 || dst.Channels != src.Channels || dst.W != src.W/2 || dst.H != src.H/2 {
		return fmt.Errorf("%w: dst must be src/2 with equal channels", ErrInvalid)
	}
	return scale.Box2to1(ctx, dst.Pix, dst.Stride, src.Pix, src.Stride, dst.W, dst.H, dst.Channels)
}

// Luma converts RGB to grey with integer BT.601 weights,
// Y = (38R + 75G + 15B + 64) >> 7. src must have 3 channels, dst 1, and both
// the same W and H. ctx is checked between rows. dst must not overlap src.
func Luma(ctx context.Context, dst, src Plane, lim Limits) error {
	if err := check(dst, src, 0, lim); err != nil {
		return err
	}
	if src.Channels != 3 || dst.Channels != 1 || dst.W != src.W || dst.H != src.H {
		return fmt.Errorf("%w: luma needs RGB src and grey dst of equal size", ErrInvalid)
	}
	return luma.Plane(ctx, dst.Pix, dst.Stride, src.Pix, src.Stride, src.W, src.H)
}

// Rotate90 rotates src a quarter turn (clockwise or not) into dst, which must
// be src.H by src.W with the same Channels. ctx is checked between tile rows.
// dst must not overlap src.
func Rotate90(ctx context.Context, dst, src Plane, clockwise bool, lim Limits) error {
	if err := check(dst, src, 0, lim); err != nil {
		return err
	}
	if dst.Channels != src.Channels || dst.W != src.H || dst.H != src.W {
		return fmt.Errorf("%w: dst must be src transposed with equal channels", ErrInvalid)
	}
	return rotate.Quarter(ctx, dst.Pix, dst.Stride, src.Pix, src.Stride, src.W, src.H, src.Channels, clockwise)
}

// Resample scales src to dst's size with a separable Catmull-Rom filter, Q14
// taps and int32 accumulation, horizontal pass first. The filter widens when
// downscaling. It allocates an intermediate plane and the tap tables, bounded
// by Limits.MaxScratchBytes before allocation. ctx is checked between rows.
// dst must not overlap src.
func Resample(ctx context.Context, dst, src Plane, lim Limits) error {
	if err := src.validate("src"); err != nil {
		return err
	}
	if err := dst.validate("dst"); err != nil {
		return err
	}
	if dst.Channels != src.Channels {
		return fmt.Errorf("%w: channel mismatch", ErrInvalid)
	}
	scratch := resample.ScratchBytes(src.W, src.H, dst.W, dst.H, src.Channels)
	if err := check(dst, src, scratch, lim); err != nil {
		return err
	}
	return resample.Run(ctx, dst.Pix, dst.Stride, dst.W, dst.H, src.Pix, src.Stride, src.W, src.H, src.Channels)
}

// Subsample1bpp downscales (or scales) a packed 1-bpp src to dst's size by
// nearest sample: dst pixel x takes the source pixel at (2x+1)*src.W/(2*dst.W),
// likewise for rows, integer only, with no option for the phase. Pad bits of
// each dst row's last byte are written zero. It allocates one int32 per dst
// column, bounded by Limits.MaxScratchBytes before allocation; the pixel cap
// applies to the larger of src and dst and the output cap to dst's packed
// bytes. ctx is checked between rows. dst must not overlap src.
func Subsample1bpp(ctx context.Context, dst, src Bitmap, lim Limits) error {
	if err := src.validate("src"); err != nil {
		return err
	}
	if err := dst.validate("dst"); err != nil {
		return err
	}
	if src.W > math.MaxInt32 { // the x-index table is int32
		return fmt.Errorf("%w: src: width above MaxInt32", ErrInvalid)
	}
	out := int64(dst.H) * int64((dst.W+7)/8)
	if err := checkLimits(int64(src.W)*int64(src.H), int64(dst.W)*int64(dst.H), out, subsample.ScratchBytes(dst.W), lim); err != nil {
		return err
	}
	return subsample.Run(ctx, dst.Pix, dst.Stride, dst.W, dst.H, src.Pix, src.Stride, src.W, src.H)
}

// Impl reports the implementation tier a kernel dispatches to on this host:
// "avx2", "neon" or "generic". Kernels are "scale2to1", "luma", "resample",
// "rotate90" and "subsample1bpp"; an unknown name returns "". The tier is for
// the 1-channel path of scale2to1; RGB scale2to1 is always generic.
func Impl(kernel string) string {
	switch kernel {
	case "scale2to1":
		return scale.Impl()
	case "luma":
		return luma.Impl()
	case "resample", "rotate90", "subsample1bpp":
		return "generic"
	}
	return ""
}
