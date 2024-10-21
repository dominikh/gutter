// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type BASETable struct {
	data []byte

	MajorVersion    uint16 `gen:"1"`
	MinorVersion    uint16
	horizAxisOffset opentype.Offset16[AxisTable]
	vertAxisOffset  opentype.Offset16[AxisTable]

	_                  versionDelimiter `gen:"1.1"`
	itemVarStoreOffset opentype.Offset32[ItemVariationStoreTable]
}

type AxisTable struct {
	data []byte

	baseTagListOffset    opentype.Offset16[BaseTagListTable]
	baseScriptListOffset opentype.Offset16[BaseScriptListTable]
}

type BaseTagListTable struct {
	baseTagCount uint16
	baselineTags opentype.Slice[opentype.Tag]
}

type BaseScriptListTable struct {
	data []byte

	baseScriptCount   uint16
	baseScriptRecords opentype.Slice[BaseScriptRecord]
}

type BaseScriptRecord struct {
	parentData []byte

	Tag              opentype.Tag
	baseScriptOffset opentype.Offset16[BaseScriptTable]
}

type BaseScriptTable struct {
	data []byte

	baseValuesOffset    opentype.Offset16[BaseValuesTable]
	DefaultMinMaxOffset opentype.Offset16[MinMaxTable]
	baseLangSysCount    uint16
	baseLangSysRecords  opentype.Slice[BaseLangSysRecord]
}

type BaseLangSysRecord struct {
	parentData []byte

	Tag          opentype.Tag
	minMaxOffset opentype.Offset16[MinMaxTable]
}

type BaseValuesTable struct {
	data []byte

	DefaultBaselineIndex uint16
	baseCoordCount       uint16
	baseCoordOffsets     opentype.Slice[opentype.Offset16[BaseCoordTable]]
}

type MinMaxTable struct {
	data []byte

	minCoordOffset    opentype.Offset16[BaseCoordTable]
	maxCoordOffset    opentype.Offset16[BaseCoordTable]
	featMinMaxCount   uint16
	featMinMaxRecords opentype.Slice[FeatMinMaxRecord]
}

type FeatMinMaxRecord struct {
	data []byte

	FeatureTableTag opentype.Tag
	minCoordOffset  opentype.Offset16[BaseCoordTable]
	maxCoordOffset  opentype.Offset16[BaseCoordTable]
}

type BaseCoordTable struct {
	Format uint16

	Format1 BaseCoordTableFormat1 `gen:"union(Format,1)"`
	Format2 BaseCoordTableFormat2 `gen:"union(Format,2)"`
	Format3 BaseCoordTableFormat3 `gen:"union(Format,3)"`
}

type BaseCoordTableFormat1 struct {
	Coordinate int16
}

type BaseCoordTableFormat2 struct {
	Coordinate     int16
	ReferenceGlyph uint16
	BaseCoordPoint uint16
}

type BaseCoordTableFormat3 struct {
	parentData []byte
	Coordinate int16

	// Offset to a DeviceTable or a VariationIndexTable, relative to BaseCoordTable
	DeviceOffset opentype.Offset16[any]
}
