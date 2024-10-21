// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import (
	"encoding/binary"

	"honnef.co/go/gutter/opentype"
)

type Weight uint16
type Width uint16
type FontSelection uint16

func parseWeight(buf []byte, out *Weight) int {
	*out = Weight(binary.BigEndian.Uint16(buf))
	return 0
}

func parseWidth(buf []byte, out *Width) int {
	*out = Width(binary.BigEndian.Uint16(buf))
	return 0
}

func parseFontSelection(buf []byte, out *FontSelection) int {
	*out = FontSelection(binary.BigEndian.Uint16(buf))
	return 0
}

const (
	WeightThin       Weight = 100 // thin
	WeightExtraLight Weight = 200 // extra-light
	WeightLight      Weight = 300 // light
	WeightNormal     Weight = 400 // normal
	WeightMedium     Weight = 500 // medium
	WeightSemiBold   Weight = 600 // semi-bold
	WeightBold       Weight = 700 // bold
	WeightExtraBold  Weight = 800 // extra-bold
	WeightBlack      Weight = 900 // black
)

const (
	WidthUltraCondensed Width = 1 // ultra-condensed
	WidthExtraCondensed Width = 2 // extra-condensed
	WidthCondensed      Width = 3 // condensed
	WidthSemiCondensed  Width = 4 // semi-condensed
	WidthMedium         Width = 5 // medium
	WidthSemiExpanded   Width = 6 // semi-expanded
	WidthExpanded       Width = 7 // expanded
	WidthExtraExpanded  Width = 8 // extra-expanded
	WidthUltraExpanded  Width = 9 // ultra-expanded
)

const (
	FsItalic         FontSelection = 1 << 0
	FsUnderscore     FontSelection = 1 << 1
	FsNegative       FontSelection = 1 << 2
	FsOutlined       FontSelection = 1 << 3
	FsStrikeout      FontSelection = 1 << 4
	FsBold           FontSelection = 1 << 5
	FsRegular        FontSelection = 1 << 6
	FsUseTypoMetrics FontSelection = 1 << 7
	FsWWS            FontSelection = 1 << 8
	FsOblique        FontSelection = 1 << 9
)

const (
	licenseInstallableEmbedding       = 0
	licenseRestrictedLicenseEmbedding = 2
	licensePreviewAndPrintEmbedding   = 4
	licenseEditableEmbedding          = 8
)

type OS2Table struct {
	Version             uint16
	XAvgCharWidth       int16
	UsWeightClass       Weight
	UsWidthClass        Width
	FsType              uint16
	YSubscriptXSize     int16
	YSubscriptYSize     int16
	YSubscriptXOffset   int16
	YSubscriptYOffset   int16
	YSuperscriptXSize   int16
	YSuperscriptYSize   int16
	YSuperscriptXOffset int16
	YSuperscriptYOffset int16
	YStrikeoutSize      int16
	YStrikeoutPosition  int16
	SFamilyClass        int16
	PANOSE              [10]uint8
	UlUnicodeRange      [4]uint32
	AchVendID           opentype.Tag
	FsSelection         FontSelection
	UsFirstCharIndex    uint16
	UsLastCharIndex     uint16

	_              sizeDelimiter
	STypoAscender  int16
	STypoDescender int16
	STypoLineGap   int16
	UsWinAscent    uint16
	UsWinDescent   uint16

	_                versionDelimiter `gen:"1"`
	UlCodePageRange1 [2]uint32

	_             versionDelimiter `gen:"2"`
	SxHeight      int16
	SCapHeight    int16
	UsDefaultChar uint16
	UsBreakChar   uint16
	UsMaxContext  uint16

	_                       versionDelimiter `gen:"5"`
	UsLowerOpticalPointSize uint16
	UsUpperOpticalPointSize uint16
}
