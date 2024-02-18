package render

import (
	"math"

	"gioui.org/f32"
	"gioui.org/op"
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

func (f *Flex) PerformLayout() (size f32.Point) {
	cs := f.constraints
	sizes := f.computeSizes()

	allocatedSize := sizes.allocatedSize
	actualSize := sizes.mainSize
	crossSize := sizes.crossSize

	// Align items along the main axis.
	switch f.direction {
	case Horizontal:
		size = cs.Constrain(f32.Pt(actualSize, crossSize))
		actualSize = size.X
		crossSize = size.Y
	case Vertical:
		size = cs.Constrain(f32.Pt(crossSize, actualSize))
		actualSize = size.Y
		crossSize = size.X
	}
	actualSizeDelta := actualSize - allocatedSize
	remainingSpace := max(0.0, actualSizeDelta)
	var leadingSpace float32
	var betweenSpace float32
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
			betweenSpace = remainingSpace / float32(len(f.children)-1)
		} else {
			betweenSpace = 0.0
		}
	case MainAxisAlignSpaceAround:
		if len(f.children) > 0 {
			betweenSpace = remainingSpace / float32(len(f.children))
		} else {
			betweenSpace = 0.0
		}
		leadingSpace = betweenSpace / 2.0
	case MainAxisAlignSpaceEvenly:
		if len(f.children) > 0 {
			betweenSpace = remainingSpace / float32(len(f.children)+1)
		} else {
			betweenSpace = 0.0
		}
		leadingSpace = betweenSpace
	}

	// Position elements
	childMainPosition := leadingSpace
	for _, child := range f.children {
		var childCrossPosition float32
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
			child.Handle().offset = f32.Pt(childMainPosition, childCrossPosition)
		case Vertical:
			child.Handle().offset = f32.Pt(childCrossPosition, childMainPosition)
		}
		childMainPosition += f.getMainSize(child.Handle().size) + betweenSpace
	}
	return size
}

// PerformPaint implements ObjectWithChildren.
func (f *Flex) PerformPaint(r *Renderer, ops *op.Ops) {
	for _, child := range f.children {
		stack := op.Affine(f32.Affine2D{}.Offset(child.Handle().offset)).Push(ops)
		r.Paint(child).Add(ops)
		stack.Pop()
	}
}

func (f *Flex) getMainSize(size f32.Point) float32 {
	// XXX copy Gio's Axis abstraction and methods
	switch f.direction {
	case Horizontal:
		return size.X
	case Vertical:
		return size.Y
	default:
		panic("unreachable")
	}
}

func (f *Flex) getCrossSize(size f32.Point) float32 {
	// XXX copy Gio's Axis abstraction and methods
	switch f.direction {
	case Horizontal:
		return size.Y
	case Vertical:
		return size.X
	default:
		panic("unreachable")
	}
}

func (f *Flex) computeSizes() layoutSizes {
	getFlex := func(child Object) float32 {
		d, _ := child.Handle().ParentData.(*FlexParentData)
		return d.Flex
	}
	getFit := func(child Object) FlexFit {
		d, _ := child.Handle().ParentData.(*FlexParentData)
		return d.Fit
	}

	// Determine used flex factor, size inflexible items, calculate free space.
	cs := f.constraints
	inf := float32(math.Inf(1))
	var totalFlex float32
	var maxMainSize float32
	if f.direction == Horizontal {
		maxMainSize = cs.Max.X
	} else {
		maxMainSize = cs.Max.Y
	}
	canFlex := maxMainSize < inf

	var crossSize float32
	// Sum of the sizes of the non-flexible children.
	var allocatedSize float32
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
						Min: f32.Pt(0, cs.Max.Y),
						Max: f32.Pt(inf, cs.Max.Y),
					}
				case Vertical:
					innerCs = Constraints{
						Min: f32.Pt(cs.Max.X, 0),
						Max: f32.Pt(cs.Max.X, inf),
					}
				}
			} else {
				switch f.direction {
				case Horizontal:
					innerCs = Constraints{
						Min: f32.Pt(0, 0),
						Max: f32.Pt(inf, cs.Max.Y),
					}
				case Vertical:
					innerCs = Constraints{
						Min: f32.Pt(0, 0),
						Max: f32.Pt(cs.Max.X, inf),
					}
				}
			}
			childSize := Layout(child, innerCs, true)
			allocatedSize += f.getMainSize(childSize)
			crossSize = max(crossSize, f.getCrossSize(childSize))
		}
	}

	// Distribute free space to flexible children.
	var freeSpace float32
	if canFlex {
		freeSpace = max(0.0, maxMainSize-allocatedSize)
	}
	var allocatedFlexSpace float32
	if totalFlex > 0 {
		spacePerFlex := freeSpace / float32(totalFlex)
		for _, child := range f.children {
			if flex := getFlex(child); flex > 0 {
				var maxChildExtent float32
				if canFlex {
					if child == lastFlexChild {
						maxChildExtent = freeSpace - allocatedFlexSpace
					} else {
						maxChildExtent = spacePerFlex * float32(flex)
					}
				} else {
					maxChildExtent = inf
				}
				var minChildExtent float32
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
							Min: f32.Pt(minChildExtent, cs.Max.Y),
							Max: f32.Pt(maxChildExtent, cs.Max.Y),
						}
					case Vertical:
						innerConstraints = Constraints{
							Min: f32.Pt(cs.Max.X, minChildExtent),
							Max: f32.Pt(cs.Max.X, maxChildExtent),
						}
					}
				} else {
					switch f.direction {
					case Horizontal:
						innerConstraints = Constraints{
							Min: f32.Pt(minChildExtent, 0),
							Max: f32.Pt(maxChildExtent, cs.Max.Y),
						}
					case Vertical:
						innerConstraints = Constraints{
							Min: f32.Pt(0, minChildExtent),
							Max: f32.Pt(cs.Max.X, maxChildExtent),
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

	var idealSize float32
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
	Flex float32
	Fit  FlexFit
}

type layoutSizes struct {
	mainSize      float32
	crossSize     float32
	allocatedSize float32
}
