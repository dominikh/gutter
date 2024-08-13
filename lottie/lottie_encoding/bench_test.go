// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_encoding

import (
	_ "embed"
	"testing"
)

//go:embed testdata/zipper.json
var zipperJSON []byte

func BenchmarkParse(b *testing.B) {
	for range b.N {
		if _, err := Parse(zipperJSON); err != nil {
			b.Fatal(err)
		}
	}
}
