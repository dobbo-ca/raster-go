//go:build purego || !(amd64 || arm64)

package luma

func lumaAsm(dst, src []byte, w int) int { return 0 }

// Impl names the tier in use.
func Impl() string { return "generic" }
