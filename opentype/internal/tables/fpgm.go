// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

type FpgmTable struct {
	Instructions []uint8 `gen:"slice(count=-1)"`
}
