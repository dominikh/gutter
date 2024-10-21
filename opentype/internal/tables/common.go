// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import (
	"encoding/binary"

	"honnef.co/go/gutter/opentype"
)

type ScriptListTable struct {
	data []byte

	scriptCount   uint16 `gen:"omit()"`
	scriptRecords opentype.Slice[ScriptRecord]
}

type ScriptRecord struct {
	parentData []byte

	ScriptTag    opentype.Tag
	scriptOffset opentype.Offset16[ScriptTable]
}

type ScriptTable struct {
	data []byte

	defaultLangSysOffset opentype.Offset16[LangSysTable]
	langSysCount         uint16 `gen:"omit()"`
	langSysRecords       opentype.Slice[LangSysRecord]
}

type LangSysRecord struct {
	parentData []byte

	LangSysTag    opentype.Tag
	langSysOffset opentype.Offset16[LangSysTable]
}

type LangSysTable struct {
	_reserved            opentype.Offset16[struct{}]
	RequiredFeatureIndex uint16
	featureIndexCount    uint16 `gen:"omit()"`
	featureIndices       opentype.Slice[uint16]
}

type FeatureListTable struct {
	data []byte

	featureCount   uint16 `gen:"omit()"`
	featureRecords opentype.Slice[FeatureRecord]
}

type FeatureRecord struct {
	parentData []byte

	FeatureTag    opentype.Tag
	featureOffset opentype.Offset16[FeatureTable]
}

type FeatureTable struct {
	data []byte

	// May be one of various feature parameter tables
	featureParamsOffset opentype.Offset16[any]
	lookupIndexCount    uint16 `gen:"omit()"`
	lookupListIndices   opentype.Slice[uint16]
}

type FeatureParamsCvTable struct {
	Format  uint16
	Format0 FeatureParamsCvTableFormat0 `gen:"union(Format,0)"`
}

type FeatureParamsCvTableFormat0 struct {
	FeatUILabelNameID       uint16
	FeatUITooltipTextNameID uint16
	SampleTextNameID        uint16
	NumNamedParameters      uint16
	FirstParamUILabelNameID uint16
	charCount               uint16 `gen:"omit()"`
	characters              opentype.Slice[opentype.Uint24]
}

type LookupListTable struct {
	data []byte

	lookupCount   uint16 `gen:"omit()"`
	lookupOffsets opentype.Slice[opentype.Offset16[LookupTable]]
}

type LookupTable struct {
	data []byte

	LookupType    uint16
	LookupFlag    uint16
	subTableCount uint16 `gen:"omit()"`
	// XXX figure out the right type for the offset tparam
	subtableOffsets  opentype.Slice[opentype.Offset16[any]]
	MarkFilteringSet uint16
}

type CoverageTable struct {
	data []byte

	CoverageFormat uint16
	Format1        CoverageTableFormat1 `gen:"union(CoverageFormat, 1)"`
	Format2        CoverageTableFormat2 `gen:"union(CoverageFormat, 2)"`
}

type CoverageTableFormat1 struct {
	glyphCount uint16 `gen:"omit()"`
	glyphArray opentype.Slice[uint16]
}

type CoverageTableFormat2 struct {
	data []byte

	rangeCount   uint16 `gen:"omit()"`
	rangeRecords opentype.Slice[RangeRecord]
}

type RangeRecord struct {
	StartGlyphID       uint16
	EndGlyphID         uint16
	StartCoverageIndex uint16
}

type ClassDefTable struct {
	ClassFormat uint16
	Format1     ClassDefTableFormat1 `gen:"union(ClassFormat,1)"`
	Format2     ClassDefTableFormat2 `gen:"union(ClassFormat,2)"`
}

type ClassDefTableFormat1 struct {
	StartGlyphID    uint16
	glyphCount      uint16 `gen:"omit()"`
	classValueArray opentype.Slice[uint16]
}

type ClassDefTableFormat2 struct {
	classRangeCount   uint16 `gen:"omit()"`
	classRangeRecords opentype.Slice[ClassRangeRecord]
}

type ClassRangeRecord struct {
	StartGlyphID uint16
	EndGlyphID   uint16
	Class        uint16
}

type SequenceLookupRecord struct {
	SequenceIndex   uint16
	LookupListIndex uint16
}

type SequenceContextTable struct {
	data []byte

	Format  uint16
	Format1 SequenceContextTableFormat1 `gen:"union(Format,1)"`
	Format2 SequenceContextTableFormat2 `gen:"union(Format,2)"`
	Format3 SequenceContextTableFormat3 `gen:"union(Format,3)"`
}

type SequenceContextTableFormat1 struct {
	parentData []byte

	coverageOffset    opentype.Offset16[CoverageTable]
	seqRuleSetCount   uint16 `gen:"omit()"`
	seqRuleSetOffsets opentype.Slice[opentype.Offset16[SequenceRuleSetTable]]
}

type SequenceRuleSetTable struct {
	data []byte

	seqRuleCount   uint16 `gen:"omit()"`
	seqRuleOffsets opentype.Slice[opentype.Offset16[SequenceRuleTable]]
}

type SequenceRuleTable struct {
	data []byte

	glyphCount       uint16                               `gen:"omit()"`
	seqLookupCount   uint16                               `gen:"omit()"`
	inputSequence    opentype.Slice[uint16]               `gen:"slice(count=glyphCount-1)"`
	seqLookupRecords opentype.Slice[SequenceLookupRecord] `gen:"slice(count=seqLookupCount)"`
}

type SequenceContextTableFormat2 struct {
	parentData []byte

	coverageOffset         opentype.Offset16[CoverageTable]
	classDefOffset         opentype.Offset16[ClassDefTable]
	classSeqRuleSetCount   uint16 `gen:"omit()"`
	classSeqRuleSetOffsets opentype.Slice[opentype.Offset16[ClassSequenceRuleSetTable]]
}

type ClassSequenceRuleSetTable struct {
	classSeqRuleCount   uint16 `gen:"omit()"`
	classSeqRuleOffsets opentype.Slice[opentype.Offset16[ClassSequenceRuleTable]]
}

type ClassSequenceRuleTable struct {
	glyphCount       uint16                               `gen:"omit()"`
	seqLookupCount   uint16                               `gen:"omit()"`
	inputSequence    opentype.Slice[uint16]               `gen:"slice(count=glyphCount-1)"`
	seqLookupRecords opentype.Slice[SequenceLookupRecord] `gen:"slice(count=seqLookupCount)"`
}

type SequenceContextTableFormat3 struct {
	parentData []byte

	glyphCount       uint16                                           `gen:"omit()"`
	seqLookupCount   uint16                                           `gen:"omit()"`
	coverageOffsets  opentype.Slice[opentype.Offset16[CoverageTable]] `gen:"slice(count=glyphCount)"`
	seqLookupRecords opentype.Slice[SequenceLookupRecord]             `gen:"slice(count=seqLookupCount)"`
}

type FeatureVariationsTable struct {
	data []byte

	MajorVersion                uint16 `gen:"1"`
	MinorVersion                uint16
	featureVariationRecordCount uint32 `gen:"omit()"`
	featureVariationRecords     opentype.Slice[FeatureVariationRecord]
}

type FeatureVariationRecord struct {
	parentData []byte

	conditionSetOffset             opentype.Offset32[ConditionSetTable]
	featureTableSubstitutionOffset opentype.Offset32[FeatureTableSubstitutionTable]
}

type ConditionSetTable struct {
	data []byte

	conditionCount uint16 `gen:"omit()"`
	conditions     opentype.Slice[opentype.Offset32[ConditionTable]]
}

type ConditionTable struct {
	Format  uint16
	Format1 ConditionTableFormat1 `gen:"union(Format,1)"`
}

type ConditionTableFormat1 struct {
	axisIndex           uint16
	FilterRangeMinValue opentype.Int2_14
	FilterRangeMaxValue opentype.Int2_14
}

type FeatureTableSubstitutionTable struct {
	data []byte

	MajorVersion      uint16 `gen:"1"`
	MinorVersion      uint16
	substitutionCount uint16 `gen:"omit()"`
	substitutions     opentype.Slice[FeatureTableSubstitutionRecord]
}

type FeatureTableSubstitutionRecord struct {
	parentData []byte

	FeatureIndex           uint16
	alternateFeatureOffset opentype.Offset32[FeatureTable]
}

type DeviceOrVariationIndexTable struct {
	DeltaFormat    uint16
	Device         DeviceTable
	VariationIndex VariationIndexTable
}

func ParseDeviceOrVariationIndexTable(data []byte, out *DeviceOrVariationIndexTable) int {
	*out = DeviceOrVariationIndexTable{}
	out.DeltaFormat = binary.BigEndian.Uint16(data[4:6])
	switch out.DeltaFormat {
	case 1, 2, 3:
		return ParseDeviceTable(data, &out.Device)
	case 0x8000:
		return ParseVariationIndexTable(data, &out.VariationIndex)
	default:
		// XXX return error
		return 0
	}
}

type DeviceTable struct {
	StartSize   uint16
	EndSize     uint16
	DeltaFormat uint16
	deltaValues opentype.Slice[uint16] `gen:"slice(count=-1)"`
}

type VariationIndexTable struct {
	DeltaSetOuterIndex uint16
	DeltaSetInnerIndex uint16
	DeltaFormat        uint16
}
