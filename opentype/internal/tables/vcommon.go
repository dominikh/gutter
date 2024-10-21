// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type ItemVariationStoreTable struct {
	data []byte

	Format                    uint16
	variationRegionListOffset opentype.Offset32[VariationRegionList]
	itemVariationDataCount    uint16 `gen:"omit()"`
	itemVariationDataOffsets  opentype.Slice[opentype.Offset32[ItemVariationDataSubtable]]
}

type ItemVariationDataSubtable struct {
	ItemCount        uint16
	WordDeltaCount   uint16
	regionIndexCount uint16                 `gen:"omit()"`
	regionIndexes    opentype.Slice[uint16] `gen:"slice(count=regionIndexCount,singular=RegionIndex)"`
	DeltaSets        []byte                 `gen:"slice(count=-1)"`
}

type VariationRegionList struct {
	AxisCount        uint16
	RegionCount      uint16
	variationRegions opentype.Slice[RegionAxisCoordinates] `gen:"slice(2dcount=AxisCount;RegionCount)"`
}

type RegionAxisCoordinates struct {
	StartCoord opentype.Int2_14
	PeakCoord  opentype.Int2_14
	EndCoord   opentype.Int2_14
}
