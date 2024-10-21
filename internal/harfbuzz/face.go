// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
import "C"
import (
	"structs"

	"honnef.co/go/safeish"
)

type Face struct {
	_ structs.HostLayout
	c C.hb_face_t
}

func NumFaces(b *Blob) int {
	return int(C.hb_face_count(&b.c))
}

func NewFace(blob *Blob, index int) *Face {
	return safeish.Cast[*Face](C.hb_face_create(&blob.c, C.uint(index)))
}

func (f *Face) Destroy() { C.hb_face_destroy(&f.c) }

func (f *Face) MakeImmutable() { C.hb_face_make_immutable(&f.c) }

func (f *Face) Immutable() bool { return C.hb_face_is_immutable(&f.c) != 0 }

func (f *Face) UPEM() int { return int(C.hb_face_get_upem(&f.c)) }
