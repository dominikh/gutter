// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"golang.org/x/exp/constraints"
)

func satConv[D constraints.Unsigned, S ~float32 | ~float64](x S) D {
	max := ^D(0)
	if x != x || x < 0 {
		return 0
	} else if x > S(max) {
		return max
	} else {
		return D(x)
	}
}
