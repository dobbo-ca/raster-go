// Package fixture generates synthetic, deterministic test rasters. There is
// no real document data anywhere in this repository.
package fixture

// Bytes returns n pseudo-random bytes from seed (xorshift64*).
func Bytes(seed uint64, n int) []byte {
	s := seed*0x9E3779B97F4A7C15 + 1
	b := make([]byte, n)
	for i := range b {
		s ^= s >> 12
		s ^= s << 25
		s ^= s >> 27
		b[i] = byte((s * 0x2545F4914F6CDD1D) >> 56)
	}
	return b
}

// Page returns a w*h*ch raster with ramps, hard edges and noise, so every
// kernel sees flat runs, gradients and extremes.
func Page(seed uint64, w, h, ch int) []byte {
	b := Bytes(seed, w*h*ch)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < ch; c++ {
				i := (y*w+x)*ch + c
				switch (x/16 + y/16) % 5 {
				case 0:
					b[i] = 255
				case 1:
					b[i] = byte(x*3 + y*5 + c*40)
				case 2:
					b[i] = 0
				default:
					b[i] = b[i]&0x1f | byte(x+y)
				}
			}
		}
	}
	return b
}
