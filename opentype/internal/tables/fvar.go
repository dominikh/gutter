// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type FvarTable struct {
	MajorVersion uint16 `gen:"1"`
	MinorVersion uint16

	axesArrayOffset uint16 `gen:"omit()"`
	_reserved       uint16
	axisCount       uint16 `gen:"omit()"`
	axisSize        uint16
	instanceCount   uint16 `gen:"omit()"`
	instanceSize    uint16

	axes opentype.Slice[VariationAxisRecord] `gen:"slice(offset=axesArrayOffset, count=axisCount, size=axisSize, singular=Axis)"`

	// XXX parse instances
	// instances opentype.Slice[InstanceRecord]      `gen:"slice(offset=int(axesArrayOffset)+len(out.axes), count=instanceCount, size=instanceSize)"`
}

type VariationAxisRecord struct {
	Tag          opentype.Tag
	MinValue     opentype.Int16_16
	DefaultValue opentype.Int16_16
	MaxValue     opentype.Int16_16
	Flags        uint16
	NameID       opentype.NameID
}

// type InstanceRecord struct {
// 	Subfamily   opentype.NameID
// 	Flags       uint16
// 	Coordinates UserTuple

// 	_          sizeDelimiter
// 	PostScript opentype.NameID
// }

// type UserTuple struct {
// 	// XXX axisCount from FvarTable
// 	coordinates opentype.Slice[opentype.Int16_16] `gen:"slice(count=axisCount)"`
// }
