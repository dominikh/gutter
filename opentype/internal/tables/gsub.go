// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

const (
	GSUBLookTypeSingle                       = 1
	GSUBLookTypeMultiple                     = 2
	GSUBLookTypeAlternate                    = 3
	GSUBLookTypeLigature                     = 4
	GSUBLookTypeContext                      = 5
	GSUBLookTypeChainingContext              = 6
	GSUBLookTypeExtensionSubstitution        = 7
	GSUBLookTypeReverseChainingContextSingle = 8
)

type GSUBTable struct {
	data []byte

	MajorVersion      uint16 `gen:"1"`
	MinorVersion      uint16
	scriptListOffset  opentype.Offset16[ScriptListTable]
	featureListOffset opentype.Offset16[FeatureListTable]
	lookupListOffset  opentype.Offset16[LookupListTable]

	_                       versionDelimiter `gen:"1.1"`
	featureVariationsOffset opentype.Offset32[FeatureVariationsTable]
}

type SingleSubstTable struct {
	SubstFormat uint16
	Format1     SingleSubstTableFormat1 `gen:"union(SubstFormat,1)"`
	Format2     SingleSubstTableFormat2 `gen:"union(SubstFormat,2)"`
}

type SingleSubstTableFormat1 struct {
	parentData []byte

	coverageOffset opentype.Offset16[CoverageTable]
	DeltaGlyphID   uint16
}

type SingleSubstTableFormat2 struct {
	parentData []byte

	coverageOffset     opentype.Offset16[CoverageTable]
	glyphCount         uint16
	substituteGlyphIDs opentype.Slice[uint16]
}

type MultipleSubstTable struct {
	SubstFormat uint16
	Format1     MultipleSubstTableFormat1 `gen:"union(SubstFormat,1)"`
}

type MultipleSubstTableFormat1 struct {
	parentData []byte

	coverageOffset  opentype.Offset16[CoverageTable]
	sequenceCount   uint16 `gen:"omit()"`
	sequenceOffsets opentype.Slice[opentype.Offset16[SequenceTable]]
}

type SequenceTable struct {
	glyphCount         uint16 `gen:"omit()"`
	substituteGlyphIDs opentype.Slice[uint16]
}

type AlternateSubstTable struct {
	SubstFormat uint16
	Format1     AlternateSubstTableFormat1 `gen:"union(SubstFormat,1)"`
}

type AlternateSubstTableFormat1 struct {
	parentData []byte

	coverageOffset      opentype.Offset16[CoverageTable]
	alternateSetCount   uint16 `gen:"omit()"`
	alternateSetOffsets opentype.Slice[opentype.Offset16[AlternateSetTable]]
}

type AlternateSetTable struct {
	glyphCount        uint16 `gen:"omit()"`
	alternateGlyphIDs opentype.Slice[uint16]
}

type LigatureSubstTable struct {
	SubstFormat uint16
	Format1     LigatureSubstTableFormat1 `gen:"union(SubstFormat,1)"`
}

type LigatureSubstTableFormat1 struct {
	parentData []byte

	coverageOffset     opentype.Offset16[CoverageTable]
	ligatureSetCount   uint16 `gen:"omit()"`
	ligatureSetOffsets opentype.Slice[opentype.Offset16[LigatureSetTable]]
}

type LigatureSetTable struct {
	ligatureCount   uint16 `gen:"omit()"`
	ligatureOffsets opentype.Slice[opentype.Offset16[LigatureTable]]
}

type LigatureTable struct {
	LigatureGlyph     uint16
	componentCount    uint16 `gen:"omit()"`
	componentGlyphIDs opentype.Slice[uint16]
}

type ExtensionSubstTable struct {
	SubstFormat uint16
	Format1     ExtensionSubstTableFormat1 `gen:"union(SubstFormat,1)"`
}

type ExtensionSubstTableFormat1 struct {
	parentData []byte

	ExtensionLookupType uint16
	ExtensionOffset     opentype.Offset32[any]
}

type ReverseChainSingleSubstTable struct {
	SubstFormat uint16
	Format1     ReverseChainSingleSubstTableFormat1 `gen:"union(SubstFormat,1)"`
}

type ReverseChainSingleSubstTableFormat1 struct {
	parentData []byte

	coverageOffset           opentype.Offset16[CoverageTable]
	backtrackGlyphCount      uint16 `gen:"omit()"`
	backtrackCoverageOffsets opentype.Slice[opentype.Offset16[CoverageTable]]
	lookaheadGlyphCount      uint16 `gen:"omit()"`
	lookaheadCoverageOffsets opentype.Slice[opentype.Offset16[CoverageTable]]
	glyphCount               uint16 `gen:"omit()"`
	substituteGlyphIDs       opentype.Slice[uint16]
}
