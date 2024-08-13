// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_converter

import (
	_ "embed"
	"testing"

	"honnef.co/go/gutter/lottie/lottie_encoding"
)

//go:embed testdata/zipper.json
var zipperJSON []byte

func BenchmarkConvert(b *testing.B) {
	comp, err := lottie_encoding.Parse(zipperJSON)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for range b.N {
		ConvertAnimation(comp)
	}
}
