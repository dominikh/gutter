// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

type PCLTTable struct {
	MajorVersion        uint16 `gen:"1"`
	MinorVersion        uint16
	FontNumber          uint32
	Pitch               uint16
	XHeight             uint16
	Style               uint16
	TypeFamily          uint16
	CapHeight           uint16
	SymbolSet           uint16
	Typeface            [16]uint8
	CharacterComplement [8]uint8
	FileName            [6]uint8
	StrokeWeight        int8
	WidthType           int8
	SerifStyle          uint8
	_reserved           uint8
}
