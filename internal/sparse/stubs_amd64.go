// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

func fineFillSSE(out [][stripHeight][4]float32, color [4]float32)
func fineFillAVX(out [][stripHeight][4]float32, color [4]float32)
