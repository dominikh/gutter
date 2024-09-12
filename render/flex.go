// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/jello"
)

type FlexFit uint8

const (
	FlexFitTight FlexFit = iota
	FlexFitLoose
)

type MainAxisSize uint8

const (
	MainAxisSizeMax = iota
	MainAxisSizeMin
)

type MainAxisAlignment uint8

const (
	MainAxisAlignStart = iota
	MainAxisAlignEnd
	MainAxisAlignCenter
	MainAxisAlignSpaceBetween
	MainAxisAlignSpaceAround
	MainAxisAlignSpaceEvenly
)

type CrossAxisAlignment uint8

const (
	CrossAxisAlignCenter = iota
	CrossAxisAlignStart
	CrossAxisAlignEnd
	CrossAxisAlignStretch
	CrossAxisAlignBaseline
)

type Axis uint8

const (
	Horizontal = iota
	Vertical
)

var _ ObjectWithChildren = (*Flex)(nil)

type Flex struct {
	Box
	ManyChildren

	direction          Axis
	mainAxisSize       MainAxisSize
	mainAxisAlignment  MainAxisAlignment
	crossAxisAlignment CrossAxisAlignment
	// XXX add clip behavior
}

func (f *Flex) SetDirection(v Axis) {
	if f.direction != v {
		f.direction = v
		MarkNeedsLayout(f)
	}
}

func (f *Flex) SetMainAxisSize(v MainAxisSize) {
	if f.mainAxisSize != v {
		f.mainAxisSize = v
		MarkNeedsLayout(f)
	}
}

func (f *Flex) SetMainAxisAlignment(v MainAxisAlignment) {
	if f.mainAxisAlignment != v {
		f.mainAxisAlignment = v
		MarkNeedsLayout(f)
	}
}

func (f *Flex) SetCrossAxisAlignment(v CrossAxisAlignment) {
	if f.crossAxisAlignment != v {
		f.crossAxisAlignment = v
		MarkNeedsLayout(f)
	}
}

// PerformLayout implements ObjectWithChildren.

func (f *Flex) PerformSetupParentData(child Object) {
	child.Handle().ParentData = &FlexParentData{}
}

func (f *Flex) PerformLayout() (size curve.Size) {
	cs := f.constraints
	sizes := f.computeSizes()

	allocatedSize := sizes.allocatedSize
	actualSize := sizes.mainSize
	crossSize := sizes.crossSize

	// Align items along the main axis.
	switch f.direction {
	case Horizontal:
		size = cs.Constrain(curve.Sz(actualSize, crossSize))
		actualSize = size.Width
		crossSize = size.Height
	case Vertical:
		size = cs.Constrain(curve.Sz(crossSize, actualSize))
		actualSize = size.Height
		crossSize = size.Width
	}
	actualSizeDelta := actualSize - allocatedSize
	remainingSpace := max(0.0, actualSizeDelta)
	var leadingSpace float64
	var betweenSpace float64
	switch f.mainAxisAlignment {
	case MainAxisAlignStart:
		leadingSpace = 0.0
		betweenSpace = 0.0
	case MainAxisAlignEnd:
		leadingSpace = remainingSpace
		betweenSpace = 0.0
	case MainAxisAlignCenter:
		leadingSpace = remainingSpace / 2.0
		betweenSpace = 0.0
	case MainAxisAlignSpaceBetween:
		leadingSpace = 0.0
		if len(f.children) > 1 {
			betweenSpace = remainingSpace / float64(len(f.children)-1)
		} else {
			betweenSpace = 0.0
		}
	case MainAxisAlignSpaceAround:
		if len(f.children) > 0 {
			betweenSpace = remainingSpace / float64(len(f.children))
		} else {
			betweenSpace = 0.0
		}
		leadingSpace = betweenSpace / 2.0
	case MainAxisAlignSpaceEvenly:
		if len(f.children) > 0 {
			betweenSpace = remainingSpace / float64(len(f.children)+1)
		} else {
			betweenSpace = 0.0
		}
		leadingSpace = betweenSpace
	}

	// Position elements
	childMainPosition := leadingSpace
	for _, child := range f.children {
		var childCrossPosition float64
		switch f.crossAxisAlignment {
		case CrossAxisAlignStart:
		case CrossAxisAlignEnd:
			if f.crossAxisAlignment == CrossAxisAlignStart {
				childCrossPosition = 0.0
			} else {
				childCrossPosition = crossSize - f.getCrossSize(child.Handle().size)
			}
		case CrossAxisAlignCenter:
			childCrossPosition = crossSize/2.0 - f.getCrossSize(child.Handle().size)/2.0
		case CrossAxisAlignStretch:
			childCrossPosition = 0.0
		}
		switch f.direction {
		case Horizontal:
			child.Handle().Offset = curve.Pt(childMainPosition, childCrossPosition)
		case Vertical:
			child.Handle().Offset = curve.Pt(childCrossPosition, childMainPosition)
		}
		childMainPosition += f.getMainSize(child.Handle().size) + betweenSpace
	}
	return size
}

// PerformPaint implements ObjectWithChildren.
func (f *Flex) PerformPaint(p *Painter, scene *jello.Scene) {
	for _, child := range f.children {
		p.PaintAt(child, scene, child.Handle().Offset)
	}
}

func (f *Flex) getMainSize(size curve.Size) float64 {
	// XXX copy Gio's Axis abstraction and methods
	switch f.direction {
	case Horizontal:
		return size.Width
	case Vertical:
		return size.Height
	default:
		panic("unreachable")
	}
}

func (f *Flex) getCrossSize(size curve.Size) float64 {
	// XXX copy Gio's Axis abstraction and methods
	switch f.direction {
	case Horizontal:
		return size.Height
	case Vertical:
		return size.Width
	default:
		panic("unreachable")
	}
}

func (f *Flex) computeSizes() layoutSizes {
	getFlex := func(child Object) float64 {
		d, _ := child.Handle().ParentData.(*FlexParentData)
		return d.Flex
	}
	getFit := func(child Object) FlexFit {
		d, _ := child.Handle().ParentData.(*FlexParentData)
		return d.Fit
	}

	// Determine used flex factor, size inflexible items, calculate free space.
	cs := f.constraints
	inf := float64(math.Inf(1))
	var totalFlex float64
	var maxMainSize float64
	if f.direction == Horizontal {
		maxMainSize = cs.Max.Width
	} else {
		maxMainSize = cs.Max.Height
	}
	canFlex := maxMainSize < inf

	var crossSize float64
	// Sum of the sizes of the non-flexible children.
	var allocatedSize float64
	var lastFlexChild Object
	for _, child := range f.children {
		flex := getFlex(child)
		if flex > 0 {
			totalFlex += flex
			lastFlexChild = child
		} else {
			var innerCs Constraints
			if f.crossAxisAlignment == CrossAxisAlignStretch {
				switch f.direction {
				case Horizontal:
					innerCs = Constraints{
						Min: curve.Sz(0, cs.Max.Height),
						Max: curve.Sz(inf, cs.Max.Height),
					}
				case Vertical:
					innerCs = Constraints{
						Min: curve.Sz(cs.Max.Width, 0),
						Max: curve.Sz(cs.Max.Width, inf),
					}
				}
			} else {
				switch f.direction {
				case Horizontal:
					innerCs = Constraints{
						Min: curve.Sz(0, 0),
						Max: curve.Sz(inf, cs.Max.Height),
					}
				case Vertical:
					innerCs = Constraints{
						Min: curve.Sz(0, 0),
						Max: curve.Sz(cs.Max.Width, inf),
					}
				}
			}
			childSize := Layout(child, innerCs, true)
			allocatedSize += f.getMainSize(childSize)
			crossSize = max(crossSize, f.getCrossSize(childSize))
		}
	}

	// Distribute free space to flexible children.
	var freeSpace float64
	if canFlex {
		freeSpace = max(0.0, maxMainSize-allocatedSize)
	}
	var allocatedFlexSpace float64
	if totalFlex > 0 {
		spacePerFlex := freeSpace / float64(totalFlex)
		for _, child := range f.children {
			if flex := getFlex(child); flex > 0 {
				var maxChildExtent float64
				if canFlex {
					if child == lastFlexChild {
						maxChildExtent = freeSpace - allocatedFlexSpace
					} else {
						maxChildExtent = spacePerFlex * float64(flex)
					}
				} else {
					maxChildExtent = inf
				}
				var minChildExtent float64
				switch getFit(child) {
				case FlexFitTight:
					minChildExtent = maxChildExtent
				case FlexFitLoose:
					minChildExtent = 0.0
				}
				var innerConstraints Constraints
				if f.crossAxisAlignment == CrossAxisAlignStretch {
					switch f.direction {
					case Horizontal:
						innerConstraints = Constraints{
							Min: curve.Sz(minChildExtent, cs.Max.Height),
							Max: curve.Sz(maxChildExtent, cs.Max.Height),
						}
					case Vertical:
						innerConstraints = Constraints{
							Min: curve.Sz(cs.Max.Width, minChildExtent),
							Max: curve.Sz(cs.Max.Width, maxChildExtent),
						}
					}
				} else {
					switch f.direction {
					case Horizontal:
						innerConstraints = Constraints{
							Min: curve.Sz(minChildExtent, 0),
							Max: curve.Sz(maxChildExtent, cs.Max.Height),
						}
					case Vertical:
						innerConstraints = Constraints{
							Min: curve.Sz(0, minChildExtent),
							Max: curve.Sz(cs.Max.Width, maxChildExtent),
						}
					}
				}
				childSize := Layout(child, innerConstraints, true)
				childMainSize := f.getMainSize(childSize)
				allocatedSize += childMainSize
				allocatedFlexSpace += maxChildExtent
				crossSize = max(crossSize, f.getCrossSize(childSize))
			}
		}
	}

	var idealSize float64
	if canFlex && f.mainAxisSize == MainAxisSizeMax {
		idealSize = maxMainSize
	} else {
		idealSize = allocatedSize
	}
	return layoutSizes{
		mainSize:      idealSize,
		crossSize:     crossSize,
		allocatedSize: allocatedSize,
	}
}

type FlexParentData struct {
	Flex float64
	Fit  FlexFit
}

type layoutSizes struct {
	mainSize      float64
	crossSize     float64
	allocatedSize float64
}
