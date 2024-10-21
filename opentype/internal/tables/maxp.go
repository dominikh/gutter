// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

type MaxpTable struct {
	MajorVersion uint16 `gen:"1"`
	MinorVersion uint16 `gen:"version16dot16"`

	_         versionDelimiter `gen:"0.5"`
	NumGlyphs uint16

	_                     versionDelimiter `gen:"1.0"`
	MaxPoints             uint16
	MaxContours           uint16
	MaxCompositePoints    uint16
	MaxCompositeContours  uint16
	MaxZones              uint16
	MaxTwilightPoints     uint16
	MaxStorage            uint16
	MaxFunctionDefs       uint16
	MaxInstructionDefs    uint16
	MaxStackElements      uint16
	MaxSizeOfInstructions uint16
	MaxComponentElements  uint16
	MaxComponentDepth     uint16
}
