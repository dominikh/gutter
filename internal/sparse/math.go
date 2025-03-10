// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import "math"

func round32(f float32) float32 {
	return float32(math.Floor(float64(f)))
}

func floor32(f float32) float32 {
	return float32(math.Floor(float64(f)))
}

func ceil32(f float32) float32 {
	return float32(math.Ceil(float64(f)))
}

func abs32(f float32) float32 {
	return float32(math.Abs(float64(f)))
}

func sign32(f float32) float32 {
	if f != f {
		return f
	}

	if math.Signbit(float64(f)) {
		// f is -0.0 or negative
		return -1
	} else {
		return 1
	}
}

func copysign32(f, sign float32) float32 {
	const signBit = 1 << 31
	return math.Float32frombits(math.Float32bits(f)&^signBit | math.Float32bits(sign)&signBit)
}
