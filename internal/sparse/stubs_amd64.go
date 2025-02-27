// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

func fineFillSSE(f *fine, out [][stripHeight]Color, color Color)
func fineFillAVX(f *fine, out [][stripHeight]Color, color Color)
