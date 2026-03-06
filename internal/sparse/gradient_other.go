// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !amd64 || !goexperiment.simd || noasm

package sparse

func (gf *gradientFiller) fill(dst Pixels) {
	gf.fillScalar(dst)
}
