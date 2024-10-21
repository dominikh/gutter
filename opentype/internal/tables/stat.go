// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type STATTable struct {
	data []byte

	MajorVersion             uint16 `gen:"1"`
	MinorVersion             uint16
	designAxisSize           uint16
	designAxisCount          uint16 `gen:"omit()"`
	designAxesOffset         uint32
	axisValueCount           uint16 `gen:"omit()"`
	offsetToAxisValueOffsets uint32

	_                    versionDelimiter `gen:"1.1"`
	ElidedFallbackNameID opentype.NameID

	designAxes       opentype.Slice[AxisRecord]                        `gen:"slice(offset=designAxesOffset, count=designAxisCount, size=designAxisSize, singular=DesignAxis)"`
	axisValueOffsets opentype.Slice[opentype.Offset16[AxisValueTable]] `gen:"slice(offset=offsetToAxisValueOffsets, count=axisValueCount)"`
}

type AxisRecord struct {
	AxisTag      opentype.Tag
	AxisNameID   opentype.NameID
	AxisOrdering uint16
}

type AxisValueTable struct {
	Format  uint16
	Format1 AxisValueTableFormat1 `gen:"union(Format,1)"`
	Format2 AxisValueTableFormat2 `gen:"union(Format,2)"`
	Format3 AxisValueTableFormat3 `gen:"union(Format,3)"`
	Format4 AxisValueTableFormat4 `gen:"union(Format,4)"`
}

type AxisValueTableFormat1 struct {
	AxisIndex   uint16
	Flags       uint16
	ValueNameID opentype.NameID
	Value       opentype.Int16_16
}

type AxisValueTableFormat2 struct {
	AxisIndex     uint16
	Flags         uint16
	ValueNameID   opentype.NameID
	NominalValue  opentype.Int16_16
	RangeMinValue opentype.Int16_16
	RangeMaxValue opentype.Int16_16
}

type AxisValueTableFormat3 struct {
	AxisIndex   uint16
	Flags       uint16
	ValueNameID opentype.NameID
	Value       opentype.Int16_16
	LinkedValue opentype.Int16_16
}

type AxisValueTableFormat4 struct {
	axisCount   uint16 `gen:"omit()"`
	Flags       uint16
	ValueNameID opentype.NameID
	axisValues  opentype.Slice[AxisValue] `gen:"slice(count=axisCount)"`
}

type AxisValue struct {
	AxisIndex uint16
	Value     opentype.Int16_16
}
