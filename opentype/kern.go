// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import "iter"

func (tbl *KernTable) Subtables() iter.Seq[KernSubTable] {
	return func(yield func(KernSubTable) bool) {
		b := tbl.subtables
		for len(b) > 0 {
			var sub KernSubTable
			dyn := ParseKernSubTable(b, &sub)
			if !yield(sub) {
				return
			}
			b = b[dyn+6:]
		}
	}
}

func (tbl *KernSubTable) Format() int {
	return int((tbl.Coverage & 0xFF00) >> 8)
}

func (tbl *KernSubtableFormat2) KerningValue(leftClass, rightClass uint16) int16 {
	off := int(tbl.kerningArrayOffset) + (int(leftClass) + int(rightClass))

	var out int16
	parseInt16(tbl.data[off:], &out)
	return out
}
