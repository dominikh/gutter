// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type GlyfTable struct {
	Data []byte `gen:"slice(count=-1)"`
}

type SimpleGlyphTable struct {
	NumberOfContours int16
	XMin             int16
	YMin             int16
	XMax             int16
	YMax             int16

	endPtsOfContours  opentype.Slice[uint16] `gen:"slice(count=NumberOfContours)"`
	instructionLength uint16
	Instructions      []byte

	// Data contains arrays of flags and X and Y coordinates
	Data []byte `gen:"slice(count=-1)"`
}

type CompositeGlyphTable struct {
	NumberOfContours int16
	XMin             int16
	YMin             int16
	XMax             int16
	YMax             int16

	Flags      uint16
	GlyphIndex uint16
	Data       []byte `gen:"slice(count=-1)"`
}
