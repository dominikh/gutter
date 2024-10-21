// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type MergeFlag uint8

const (
	MergeLTR               MergeFlag = 1
	GroupLTR               MergeFlag = 2
	SecondIsSubordinateLTR MergeFlag = 4
	MergeRTL               MergeFlag = 16
	GroupRTL               MergeFlag = 32
	SecondIsSubordinateRTL MergeFlag = 64
)

type MERGTable struct {
	Version                 uint16
	MergeClassCount         uint16
	mergeDataOffset         uint16 `gen:"omit()"`
	classDefCount           uint16 `gen:"omit()"`
	offsetToClassDefOffsets uint16 `gen:"omit()"`

	classDefOffsets opentype.Slice[opentype.Offset16[ClassDefTable]] `gen:"slice(count=classDefCount, offset=offsetToClassDefOffsets)"`
	// Row-major 2D array of merge entries, MergeClassCount² many.
	MergeEntries []MergeFlag `gen:"slice(2dcount=MergeClassCount;MergeClassCount, offset=mergeDataOffset)"`
}
