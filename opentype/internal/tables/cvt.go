// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type CvtTable struct {
	values opentype.Slice[int16] `gen:"slice(count=-1)"`
}
