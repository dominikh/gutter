// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
import "C"
import "unsafe"

func Shape(font *Font, buf *Buffer, features []Feature) {
	feats := make([]C.hb_feature_t, len(features))
	for i, f := range features {
		feats[i] = C.hb_feature_t{
			tag:   C.hb_tag_t(f.Tag.Uint32()),
			value: C.uint32_t(f.Value),
			start: C.uint(f.Start),
			end:   C.uint(f.End),
		}
	}
	C.hb_shape(&font.c, &buf.c, unsafe.SliceData(feats), C.uint(len(feats)))
}
