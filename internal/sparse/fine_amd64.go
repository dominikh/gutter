// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import "golang.org/x/sys/cpu"

func init() {
	switch {
	case cpu.X86.HasAVX:
		fillSolidFp = fineFillSolidAVX
		fillSimpleFp = fineFillSimpleAVX
		fillComplexFp = fineFillComplexAVX
	case cpu.X86.HasSSE2:
		// amd64 always supports SSE and SSE2
		fillSolidFp = fineFillSolidSSE
		fillSimpleFp = fineFillSimpleSSE
		fillComplexFp = fineFillComplexSSE
	}
}
