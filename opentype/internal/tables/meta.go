// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type MetaTable struct {
	data []byte

	Version       uint32
	Flags         uint32
	_reserved     uint32
	dataMapsCount uint32 `gen:"omit()"`
	dataMaps      opentype.Slice[DataMapRecord]
}

type DataMapRecord struct {
	parentData []byte

	Tag        opentype.Tag
	dataOffset uint32 `gen:"omit()"`
	dataLength uint32 `gen:"omit()"`
	Data       []byte `gen:"slice(offset=dataOffset,count=dataLength)"`
}
