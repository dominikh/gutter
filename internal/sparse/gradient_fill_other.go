// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !amd64 || !goexperiment.simd

package sparse

func (gf *gradientFiller) fillSIMD(dst Pixels) bool {
	return false
}
