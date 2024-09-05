// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

// TODO(dh): this type probably belongs in a different package

type BoxFit int

const (
	// Scale the source to fit into the target while preserving the source's
	// aspect ratio.
	BoxFitContain BoxFit = iota

	// Scale the source to fit into the target, ignoring the source's aspect
	// ratio. The target will be completely filled.
	BoxFitFill

	// Scale the source to completely cover the target while preserving the
	// source's aspect ratio. The source may overflow the target.
	BoxFitCover

	// Set the source's width equal to the target's width and scale the source's
	// height to keep the source's aspect ratio. The source may vertically
	// overflow the target.
	BoxFitFitWidth

	// Set the source's height equal to the target's height and scale the
	// source's width to keep the source's aspect ratio. The source may
	// horizontally overflow the target.
	BoxFitFitHeight

	// The source's dimensions are used as they are, potentially overflowing the
	// target.
	BoxFitNone

	// If the source doesn't fit in the target, the source is scaled down to fit
	// into the target while preserving the source's aspect ratio. Otherwise,
	// the source's dimensions are used as they are.
	BoxFitScaleDown
)
