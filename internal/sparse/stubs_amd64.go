// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

func memsetColumnsAVX(buf [][stripHeight]Color, color Color)
func fineFillComplexAVX(buf [][stripHeight]Color, color Color)

func memsetColumnsSSE(buf [][stripHeight]Color, color Color)
func fineFillComplexSSE(buf [][stripHeight]Color, color Color)
