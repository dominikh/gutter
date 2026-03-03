// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	. "simd/archsimd"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
)

func makeGradientLUT(ranges []gradientRange) gradientLUT {
	// 11 bits of gradient accuracy. Good enough for our GUI rendering purposes.
	const lutSize = 2048
	const invLutSize = 1.0 / (lutSize - 1)
	lut := make([]gfx.PlainColor, 0, lutSize)
	curIdx := 0
	if arch.AVX2() && arch.FMA() {
		for idx := range lutSize {
			tVal := float32(idx) * invLutSize
			for ranges[curIdx].x1 < tVal {
				curIdx++
			}
			rng := &ranges[curIdx]
			bias := rng.bias
			bias_ := LoadFloat32x4(&bias)
			scale_ := LoadFloat32x4(&rng.scale)
			tVal_ := BroadcastFloat32x4(tVal)
			var interpolated [4]float32
			tVal_.MulAdd(scale_, bias_).Store(&interpolated)
			lut = append(lut, interpolated)
		}
	} else {
		for idx := range lutSize {
			tVal := float32(idx) * invLutSize
			for ranges[curIdx].x1 < tVal {
				curIdx++
			}
			rng := &ranges[curIdx]
			bias := rng.bias
			interpolated := gfx.PlainColor{
				bias[0] + rng.scale[0]*tVal,
				bias[1] + rng.scale[1]*tVal,
				bias[2] + rng.scale[2]*tVal,
				bias[3] + rng.scale[3]*tVal,
			}
			lut = append(lut, interpolated)
		}
	}

	return gradientLUT{
		lut:   lut,
		scale: lutSize - 1,
	}
}
