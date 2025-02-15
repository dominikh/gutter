// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"golang.org/x/sys/cpu"
)

func init() {
	switch {
	case cpu.X86.HasAVX:
		fillFp = fineFillAVX
	case cpu.X86.HasSSE2:
		fillFp = fineFillSSE
	}
}
