// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

type VheaTable struct {
	MajorVersion uint16 `gen:"1"`
	MinorVersion uint16 `gen:"version16dot16"`

	VertTypoAscendender  int16
	VertTypoDescender    int16
	VertTypoLineGap      int16
	AdvanceHeightMax     int16
	MinTopSideBearing    int16
	MinBottomSideBearing int16
	YMaxExtent           int16
	CaretSlopeRise       int16
	CaretSlopeRun        int16
	CaretOffset          int16
	_reserved            [4]int16
	MetricDataFormat     int16
	NumOfLongVerMetrics  uint16
}
