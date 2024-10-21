// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import (
	"time"

	"honnef.co/go/gutter/opentype"
)

type HeadTable struct {
	MajorVersion       uint16 `gen:"1"`
	MinorVersion       uint16
	FontRevision       opentype.Int16_16
	ChecksumAdjustment uint32
	MagicNumber        uint32 // has to be 0x5F0F3CF5
	Flags              uint16
	UnitsPerEm         uint16
	Created            time.Time
	Modified           time.Time
	XMin               int16
	YMin               int16
	XMax               int16
	YMax               int16
	MacStyle           uint16
	LowestRecPPEM      uint16
	FontDirectionHint  int16
	IndexToLocFormat   int16
	GlyphDataFormat    int16
}
