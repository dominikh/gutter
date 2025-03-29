// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

var (
	memsetColumnsFp             = memsetColumnsNative
	fillComplexFp               = fineFillComplexNative
	computeWindingFp            = computeWindingNative
	processOutOfBoundsWindingFp = processOutOfBoundsWindingNative
	computeAlphasNonZeroFp      = computeAlphasNonZeroNative
)
