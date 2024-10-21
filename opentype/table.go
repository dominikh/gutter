// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import (
	"cmp"
	"fmt"
	"sort"
)

func (tbl *TableDirectory) FindTable(tag Tag) (TableRecord, bool) {
	tagi := tag.Uint32()
	i, ok := sort.Find(int(tbl.NumTableRecords()), func(i int) int {
		var cur Tag
		parseTag(tbl.tableRecords[i*16:], &cur)
		return cmp.Compare(tagi, cur.Uint32())
	})
	if !ok {
		return TableRecord{}, false
	}
	return tbl.TableRecord(i), true
}

func (rec *TableRecord) Data() []byte {
	return rec.parentData[rec.Offset : int(rec.Offset)+int(rec.Length)]
}

func (rec *TableRecord) String() string {
	return fmt.Sprintf("%s %d–%d (chksum=%#x)",
		rec.Tag, rec.Offset, int(rec.Offset)+int(rec.Length), rec.Checksum)
}
