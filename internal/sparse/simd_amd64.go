// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import "honnef.co/go/gutter/internal/arch"

var hasAVX2AndFMA3 = arch.AVX2() && arch.FMA()
