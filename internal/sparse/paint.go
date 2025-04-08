// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

type Paint interface {
	encode() encodedPaint
}

func (s Color) encode() encodedPaint {
	return s
}

type encodedPaint interface {
	isEncodedPaint()
}
