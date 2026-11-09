//go:build !unix

package fixture

// Guarded has no guard page here; it is a plain slice.
func Guarded(n int) (b []byte, free func()) { return make([]byte, n), func() {} }
