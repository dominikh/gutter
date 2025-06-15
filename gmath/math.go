// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gmath

import (
	"math"

	"golang.org/x/exp/constraints"
)

func Clamp[T constraints.Integer | constraints.Float](x, minv, maxv T) T {
	return min(max(x, minv), maxv)
}

func Floor32(f float32) float32 {
	return float32(math.Floor(float64(f)))
}

func Ceil32(f float32) float32 {
	return float32(math.Ceil(float64(f)))
}

func Abs32(f float32) float32 {
	return math.Float32frombits(math.Float32bits(f) &^ (1 << 31))
}

func Sqrt32(f float32) float32 {
	return float32(math.Sqrt(float64(f)))
}

func Sign32(f float32) float32 {
	if math.Float32bits(f)&(1<<31) != 0 {
		// f is -0.0 or negative
		return -1
	} else {
		return 1
	}
}

func IsNaN(f float32) bool {
	return f != f
}
