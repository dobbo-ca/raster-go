//go:build unix

package fixture

import (
	"os"
	"syscall"
)

// Guarded returns an n-byte slice whose last byte is the last byte before a
// PROT_NONE page, so one byte of overread faults. Call free when done.
func Guarded(n int) (b []byte, free func()) {
	ps := os.Getpagesize()
	body := (n + ps - 1) / ps * ps
	m, err := syscall.Mmap(-1, 0, body+ps, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		panic(err)
	}
	if err := syscall.Mprotect(m[body:], syscall.PROT_NONE); err != nil {
		panic(err)
	}
	return m[body-n : body : body], func() { _ = syscall.Munmap(m) }
}
