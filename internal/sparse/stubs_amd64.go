// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

func fineFillSolidAVX(buf [][stripHeight]Color, color Color)
func fineFillSimpleAVX(buf [][stripHeight]Color, color Color, bg Color)
func fineFillComplexAVX(buf [][stripHeight]Color, color Color)

func fineFillSolidSSE(buf [][stripHeight]Color, color Color)
func fineFillSimpleSSE(buf [][stripHeight]Color, color Color, bg Color)
func fineFillComplexSSE(buf [][stripHeight]Color, color Color)
