// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
import "C"
import (
	"structs"
	"unsafe"

	"honnef.co/go/gutter/opentype"
	"honnef.co/go/safeish"
)

type Direction int

const (
	LTR Direction = C.HB_DIRECTION_LTR
	RTL Direction = C.HB_DIRECTION_RTL
	TTB Direction = C.HB_DIRECTION_TTB
	BTT Direction = C.HB_DIRECTION_BTT
)

type Language struct {
	_ structs.HostLayout
	c C.hb_language_t
}

type Variation struct {
	Tag   opentype.Tag
	Value float64
}

func LanguageFromString(s string) Language {
	return Language{
		c: C.hb_language_from_string(safeish.Cast[*C.char](unsafe.StringData(s)), C.int(len(s))),
	}
}

func (l Language) String() string {
	s := safeish.Cast[*byte](C.hb_language_to_string(l.c))
	return unsafe.String(s, safeish.FindNull(s))
}

type Feature struct {
	Tag   opentype.Tag
	Value uint32
	Start int
	End   int
}
