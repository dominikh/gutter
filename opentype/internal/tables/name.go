// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type NameTable struct {
	Version     uint16
	Count       uint16                     `gen:"omit()"`
	storage     opentype.Offset16[[]byte]  `gen:"omit()"`
	nameRecords opentype.Slice[NameRecord] `gen:"slice(count=Count)"`

	Data []byte `gen:"slice(offset=storage,count=-1)"`

	_              versionDelimiter `gen:"1"`
	LangTagCount   uint16           `gen:"omit()"`
	langTagRecords opentype.Slice[LangTagRecord]
}

type LangTagRecord struct {
	Length uint16
	// Offset relative to NameTablev0's storage area, which is at storageOffset.
	Offset opentype.Offset16[[]byte]
}

type NameRecord struct {
	PlatformID opentype.PlatformID
	EncodingID opentype.EncodingID
	LanguageID uint16
	NameID     opentype.NameID
	Length     uint16
	// offset relative to NameTablev0's storage area, which is at storageOffset.
	StringOffset uint16
}
