// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !amd64 || !goexperiment.simd

package sparse

import "honnef.co/go/gutter/gfx"

func makeGradientLUT(ranges []gradientRange) gradientLUT {
	// 11 bits of gradient accuracy. Good enough for our GUI rendering purposes.
	const lutSize = 2048
	const invLutSize = 1.0 / (lutSize - 1)
	lut := make([]gfx.PlainColor, 0, lutSize)
	curIdx := 0
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

	return gradientLUT{
		lut:   lut,
		scale: lutSize - 1,
	}
}
