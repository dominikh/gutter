// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type TableDirectory struct {
	data []byte

	SfntVersion uint32
	numTables   uint16 `gen:"omit()"`
	// The following 3 fields shouldn't be used and should instead be derived from NumTables.
	_searchRange   uint16
	_entrySelector uint16
	_rangeShift    uint16
	// Followed by numTables rawTableRecords, sorted in ascending order by tag.
	tableRecords opentype.Slice[TableRecord] `gen:"slice(count=numTables)"`
}

type TableRecord struct {
	parentData []byte

	Tag      opentype.Tag
	Checksum uint32
	Offset   opentype.Offset32[any]
	Length   uint32
}
