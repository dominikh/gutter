// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type DSIGTable struct {
	data []byte

	Version          uint32
	numSignatures    uint16 `gen:"omit()"`
	Flags            uint16
	signatureRecords opentype.Slice[SignatureRecord]
}

type SignatureRecord struct {
	parentData []byte

	Format        uint32
	Length        uint32
	format1Offset opentype.Offset32[SignatureBlockFormat1] `gen:"union(Format,1)"`
}

type SignatureBlockFormat1 struct {
	_reserved1      uint16
	_reserved2      uint16
	signatureLength uint32
	Signature       []byte `gen:"slice(count=signatureLength)"`
}
