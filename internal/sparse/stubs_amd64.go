// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import "honnef.co/go/gutter/gfx"

//go:noescape
func memsetColumnsAVX(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor)

//go:noescape
func fineFillComplexAVX(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor)

//go:noescape
func memsetColumnsSSE(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor)

//go:noescape
func fineFillComplexSSE(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor)

//go:noescape
func computeWindingSSE(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
)

//go:noescape
func computeWindingAVX(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
)

//go:noescape
func computeWindingAVXFMA(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
)

//go:noescape
func processOutOfBoundsWindingSSE(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
)

//go:noescape
func computeAlphasNonZeroSSE(tail *[tileWidth][tileHeight]uint8, locationWinding *[tileWidth][tileHeight]float32)

//go:noescape
func computeAlphasNonZeroAVX(tail *[tileWidth][tileHeight]uint8, locationWinding *[tileWidth][tileHeight]float32)
