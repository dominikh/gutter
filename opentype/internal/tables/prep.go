// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

type PrepTable struct {
	Data []byte `gen:"slice(count=-1)"`
}
