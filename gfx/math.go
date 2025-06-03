// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gfx

import "math"

func sqrt32(f float32) float32 {
	return float32(math.Sqrt(float64(f)))
}

func abs32(f float32) float32 {
	return math.Float32frombits(math.Float32bits(f) &^ (1 << 31))
}
