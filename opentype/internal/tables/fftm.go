// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "time"

type FFTMTable struct {
	Version          uint32
	Fontforge        time.Time
	FontCreation     time.Time
	FontModification time.Time
}
