package raster_test

import (
	"fmt"
	"testing"

	raster "github.com/dobbo-ca/raster-go"
	"github.com/dobbo-ca/raster-go/internal/fixture"
)

const bw, bh = 2550, 3300 // a letter page at 300 dpi

func benchPlane(w, h, ch int) raster.Plane {
	return raster.Plane{Pix: fixture.Page(1, w, h, ch), W: w, H: h, Stride: w * ch, Channels: ch}
}

func BenchmarkScale2to1(b *testing.B) {
	for _, tc := range []struct {
		name string
		ch   int
	}{{"gray", 1}, {"rgb", 3}} {
		b.Run(fmt.Sprintf("%s/%dx%d", tc.name, bw, bh), func(b *testing.B) {
			src, dst := benchPlane(bw, bh, tc.ch), benchPlane(bw/2, bh/2, tc.ch)
			b.SetBytes(int64(len(src.Pix)))
			for b.Loop() {
				if err := raster.Scale2to1(bg, dst, src, raster.Limits{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLuma(b *testing.B) {
	src, dst := benchPlane(bw, bh, 3), benchPlane(bw, bh, 1)
	b.SetBytes(int64(len(src.Pix)))
	for b.Loop() {
		if err := raster.Luma(bg, dst, src, raster.Limits{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResample(b *testing.B) {
	src, dst := benchPlane(bw, bh, 1), benchPlane(bw/2, bh/2, 1)
	b.Run("300to150", func(b *testing.B) {
		b.SetBytes(int64(len(src.Pix)))
		for b.Loop() {
			if err := raster.Resample(bg, dst, src, raster.Limits{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkRotate90(b *testing.B) {
	src, dst := benchPlane(bw, bh, 1), benchPlane(bh, bw, 1)
	b.SetBytes(int64(len(src.Pix)))
	for b.Loop() {
		if err := raster.Rotate90(bg, dst, src, true, raster.Limits{}); err != nil {
			b.Fatal(err)
		}
	}
}
