package raster_test

import (
	"context"
	"fmt"

	raster "github.com/dobbo-ca/raster-go"
)

func ExampleScale2to1() {
	src := raster.Plane{Pix: []byte{10, 20, 30, 40, 50, 60, 70, 80}, W: 4, H: 2, Stride: 4, Channels: 1}
	dst := raster.Plane{Pix: make([]byte, 2), W: 2, H: 1, Stride: 2, Channels: 1}
	if err := raster.Scale2to1(context.Background(), dst, src, raster.Limits{}); err != nil {
		panic(err)
	}
	fmt.Println(dst.Pix[0], dst.Pix[1])
	// Output: 35 55
}
