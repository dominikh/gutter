// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

func fineFillSSE(out [][stripHeight]Color, color Color)
func fineFillAVX(out [][stripHeight]Color, color Color)
