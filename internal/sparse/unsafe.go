package sparse

import "unsafe"

//go:nosplit
//go:nocheckptr
func noEscape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}
