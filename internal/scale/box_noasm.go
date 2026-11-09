//go:build purego || !(amd64 || arm64)

package scale

func boxGray(dst, r0, r1 []byte, dw int) int { return 0 }

// Impl names the tier of the grey path.
func Impl() string { return "generic" }
