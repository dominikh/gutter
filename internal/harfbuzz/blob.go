// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
// #include <stdlib.h>
import "C"
import (
	"structs"
	"unsafe"

	"honnef.co/go/safeish"
)

type Blob struct {
	_ structs.HostLayout
	c C.hb_blob_t
}

func NewBlobFromFile(path string) *Blob {
	cstr := C.CString(path)
	defer C.free(unsafe.Pointer(cstr))
	return safeish.Cast[*Blob](C.hb_blob_create_from_file(cstr))
}

func (b *Blob) SubBlob(offset, length int) *Blob {
	return safeish.Cast[*Blob](C.hb_blob_create_sub_blob(&b.c, C.uint(offset), C.uint(length)))
}

func (b *Blob) Destroy() {
	C.hb_blob_destroy(&b.c)
}
