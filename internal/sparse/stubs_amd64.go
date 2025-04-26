// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

//go:noescape
func memsetColumnsAVX(buf [][stripHeight]plainColor, color plainColor)

//go:noescape
func fineFillComplexAVX(buf [][stripHeight]plainColor, color plainColor)

//go:noescape
func memsetColumnsSSE(buf [][stripHeight]plainColor, color plainColor)

//go:noescape
func fineFillComplexSSE(buf [][stripHeight]plainColor, color plainColor)

//go:noescape
func computeWindingSSE(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[4][4]float32,
	accumulatedWinding *[4]float32,
)

//go:noescape
func computeWindingAVX(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[4][4]float32,
	accumulatedWinding *[4]float32,
)

//go:noescape
func computeWindingAVXFMA(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[4][4]float32,
	accumulatedWinding *[4]float32,
)

//go:noescape
func processOutOfBoundsWindingSSE(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[4][4]float32,
	accumulatedWinding *[4]float32,
)

//go:noescape
func computeAlphasNonZeroSSE(tail *[4][4]uint8, locationWinding *[4][4]float32)

//go:noescape
func computeAlphasNonZeroAVX(tail *[4][4]uint8, locationWinding *[4][4]float32)
