// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
// #cgo pkg-config: harfbuzz
import "C"

func init() {
	// The first time hb_language_get_default gets called, it calls setlocale,
	// which is not thread-safe. Call it as early as possible to avoid a race
	// with other C libraries.
	C.hb_language_get_default()
}
