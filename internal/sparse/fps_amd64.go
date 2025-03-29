// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import "golang.org/x/sys/cpu"

func init() {
	memsetColumnsFp = memsetColumnsSSE
	fillComplexFp = fineFillComplexSSE
	computeWindingFp = computeWindingSSE
	processOutOfBoundsWindingFp = processOutOfBoundsWindingSSE
	computeAlphasNonZeroFp = computeAlphasNonZeroSSE

	if cpu.X86.HasAVX {
		memsetColumnsFp = memsetColumnsAVX
		fillComplexFp = fineFillComplexAVX
		computeWindingFp = computeWindingAVX
	}
	if cpu.X86.HasAVX && cpu.X86.HasFMA {
		computeWindingFp = computeWindingAVXFMA
	}
	if cpu.X86.HasAVX && cpu.X86.HasAVX2 {
		computeAlphasNonZeroFp = computeAlphasNonZeroAVX
	}
}
