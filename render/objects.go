// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"math"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/base"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/lottie/lottie_model"
	"honnef.co/go/gutter/lottie/lottie_renderer"
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/jello"
	"honnef.co/go/jello/gfx"
)

var _ Object = (*FillColor)(nil)
var _ Object = (*Lottie)(nil)

var _ ObjectWithChildren = (*Clip)(nil)
var _ ObjectWithChildren = (*Constrained)(nil)
var _ ObjectWithChildren = (*FittedBox)(nil)
var _ ObjectWithChildren = (*Opacity)(nil)
var _ ObjectWithChildren = (*AnimatedOpacity)(nil)
var _ ObjectWithChildren = (*Padding)(nil)
var _ ObjectWithChildren = (*PositionedBox)(nil)

type Box struct {
	ObjectHandle
}

// Clip prevents its child from painting outside its bounds.
type Clip struct {
	Box
	SingleChild
}

// PerformLayout implements Object.
func (w *Clip) PerformLayout() curve.Size {
	Layout(w.Child, w.Handle().Constraints(), true)
	return w.Child.Handle().Size()
}

// PerformPaint implements Object.
func (w *Clip) PerformPaint(p *Painter, scene *jello.Scene) {
	scene.PushLayer(
		gfx.BlendMode{},
		1,
		curve.Identity,
		curve.NewRectFromPoints(curve.Pt(0, 0), curve.Point(w.Handle().Size().AsVec2())).Path(0.1),
	)
	defer scene.PopLayer()
	p.PaintAt(w.Child, scene, curve.Point{})
}

// FillColor fills an infinite plane with the provided color.
//
// In layout, it takes up the least amount of space possible.
type FillColor struct {
	Box
	color color.Color
}

func (fc *FillColor) SetColor(c color.Color) {
	if fc.color != c {
		fc.color = c
		MarkNeedsPaint(fc)
	}
}

func (fc *FillColor) Color() color.Color {
	return fc.color
}

// PerformLayout implements Object.
func (c *FillColor) PerformLayout() curve.Size {
	return c.Handle().Constraints().Min
}

func (c *FillColor) SizedByParent() {}

// PerformPaint implements Object.
func (c *FillColor) PerformPaint(_ *Painter, scene *jello.Scene) {
	scene.Fill(
		gfx.NonZero,
		curve.Identity,
		gfx.SolidBrush{Color: c.color},
		curve.Identity,
		curve.NewRectFromPoints(curve.Pt(-1e9, -1e9), curve.Pt(1e9, 1e9)).Path(0.1),
	)
}

type Inset struct {
	Left, Top, Right, Bottom float64
}

func NewInset(v float64) Inset {
	return Inset{v, v, v, v}
}

func LerpInset(start, end Inset, t float64) Inset {
	return Inset{
		Left:   animation.Lerp(start.Left, end.Left, t),
		Top:    animation.Lerp(start.Top, end.Top, t),
		Right:  animation.Lerp(start.Right, end.Right, t),
		Bottom: animation.Lerp(start.Bottom, end.Bottom, t),
	}
}

type Padding struct {
	Box
	SingleChild
	inset Inset
}

func NewPadding(padding Inset) *Padding {
	return &Padding{inset: padding}
}

func (pad *Padding) SetInset(ins Inset) {
	if pad.inset != ins {
		pad.inset = ins
		MarkNeedsLayout(pad)
	}
}

func (pad *Padding) Inset() Inset {
	return pad.inset
}

// PerformLayout implements Object.
func (pad *Padding) PerformLayout() curve.Size {
	cs := pad.Handle().Constraints()
	if pad.Child == nil {
		return cs.Constrain(curve.Sz(pad.inset.Left+pad.inset.Right, pad.inset.Top+pad.inset.Bottom))
	}
	horiz := pad.inset.Left + pad.inset.Right
	vert := pad.inset.Top + pad.inset.Bottom
	newMin := curve.Sz(max(0, cs.Min.Width-horiz), max(0, cs.Min.Height-vert))
	innerCs := Constraints{
		Min: newMin,
		Max: curve.Sz(max(newMin.Width, cs.Max.Width-horiz), max(newMin.Height, cs.Max.Height-vert)),
	}
	childSz := Layout(pad.Child, innerCs, true)
	pad.Child.Handle().Offset = curve.Pt(pad.inset.Left, pad.inset.Top)
	return cs.Constrain(childSz.Add(curve.Vec(horiz, vert)))
}

// PerformPaint implements Object.
func (pad *Padding) PerformPaint(p *Painter, scene *jello.Scene) {
	p.PaintAt(pad.Child, scene, pad.Child.Handle().Offset)
}

// TODO(dh): Alignment should eventually move to a lower level package

var (
	BottomCenter = Alignment{0, 1}
	BottomLeft   = Alignment{-1, 1}
	BottomRight  = Alignment{1, 1}
	Center       = Alignment{0, 0}
	CenterLeft   = Alignment{-1, 0}
	CenterRight  = Alignment{1, 0}
	TopCenter    = Alignment{0, -1}
	TopLeft      = Alignment{-1, -1}
	TopRight     = Alignment{1, -1}
)

type Alignment struct {
	// TODO(dh): take text direction into consideration
	X, Y float64
}

func LerpAlignment(a, b Alignment, t float64) Alignment {
	return Alignment{
		X: animation.Lerp(a.X, b.X, t),
		Y: animation.Lerp(a.Y, b.Y, t),
	}
}

func (a Alignment) Inscribe(sz curve.Size, rect curve.Rect) curve.Rect {
	halfWidthDelta := (rect.Width() - sz.Width) / 2.0
	halfHeightDelta := (rect.Height() - sz.Height) / 2.0
	return curve.NewRectFromOrigin(
		curve.Pt(
			rect.X0+halfWidthDelta+a.X*halfWidthDelta,
			rect.Y0+halfHeightDelta+a.Y*halfHeightDelta,
		),
		sz,
	)
}

func (a Alignment) AlongVec2(v curve.Vec2) curve.Vec2 {
	centerX := v.X / 2.0
	centerY := v.Y / 2.0
	return curve.Vec(
		centerX+a.X*centerX,
		centerY+a.Y*centerY,
	)
}

type PositionedBox struct {
	Box
	SingleChild

	alignment    Alignment
	widthFactor  maybe.Option[float64]
	heightFactor maybe.Option[float64]
}

func (p *PositionedBox) SetAlignment(a Alignment) {
	if a != p.alignment {
		p.alignment = a
		MarkNeedsLayout(p)
	}
}

func (p *PositionedBox) SetWidthFactor(f maybe.Option[float64]) {
	if f != p.widthFactor {
		p.widthFactor = f
		MarkNeedsLayout(p)
	}
}

func (p *PositionedBox) SetHeightFactor(f maybe.Option[float64]) {
	if f != p.heightFactor {
		p.heightFactor = f
		MarkNeedsLayout(p)
	}
}

// PerformLayout implements ObjectWithChildren.
func (p *PositionedBox) PerformLayout() (size curve.Size) {
	cs := p.Constraints()
	shrinkWrapWidth := p.widthFactor.Set() || cs.Max.Width == math.Inf(1)
	shrinkWrapHeight := p.heightFactor.Set() || cs.Max.Height == math.Inf(1)

	if p.Child == nil {
		w := math.Inf(1)
		h := math.Inf(1)
		if shrinkWrapWidth {
			w = 0
		}
		if shrinkWrapHeight {
			h = 0
		}
		return cs.Constrain(curve.Sz(w, h))
	}

	childSz := Layout(p.Child, cs.Loosen(), true)
	w := math.Inf(1)
	h := math.Inf(1)
	if shrinkWrapWidth {
		w = childSz.Width * p.widthFactor.UnwrapOr(1)
	}
	if shrinkWrapHeight {
		h = childSz.Height * p.heightFactor.UnwrapOr(1)
	}
	size = cs.Constrain(curve.Sz(w, h))
	p.alignChild(size, childSz)
	return size
}

func (p *PositionedBox) alignChild(ourSize, childSize curve.Size) {
	debug.Assert(p.Child != nil)
	debug.Assert(!p.Child.Handle().needsLayout)

	p.Child.Handle().Offset = curve.Point(p.alignment.AlongVec2(ourSize.AsVec2().Add(childSize.AsVec2().Negate())))
}

// PerformPaint implements ObjectWithChildren.
func (pb *PositionedBox) PerformPaint(p *Painter, scene *jello.Scene) {
	debug.Assert(pb.Child != nil)
	p.PaintAt(pb.Child, scene, pb.Child.Handle().Offset)
}

type Constrained struct {
	Box
	SingleChild
	extraConstraints Constraints
}

func (c *Constrained) SetExtraConstraints(cs Constraints) {
	if c.extraConstraints != cs {
		c.extraConstraints = cs
		MarkNeedsLayout(c)
	}
}

func (c *Constrained) ExtraConstraints() Constraints {
	return c.extraConstraints
}

// PerformLayout implements Object.
func (c *Constrained) PerformLayout() curve.Size {
	cs := c.extraConstraints.Enforce(c.Handle().Constraints())
	if c.Child != nil {
		Layout(c.Child, cs, true)
		return c.Child.Handle().Size()
	} else {
		return cs.Min
	}
}

// PerformPaint implements Object.
func (c *Constrained) PerformPaint(p *Painter, scene *jello.Scene) {
	if c.Child != nil {
		p.PaintAt(c.Child, scene, curve.Point{})
	}
}

type Opacity struct {
	Box
	SingleChild
	opacity float32
}

// PerformLayout implements Object.
func (o *Opacity) PerformLayout() curve.Size {
	if o.Child != nil {
		return Layout(o.Child, o.constraints, true)
	} else {
		return o.constraints.Constrain(curve.Sz(0, 0))
	}
}

// PerformPaint implements Object.
func (o *Opacity) PerformPaint(p *Painter, scene *jello.Scene) {
	switch o.opacity {
	case 0:
		return
	case 1:
		p.PaintAt(o.Child, scene, curve.Point{})
	default:
		scene.PushLayer(
			gfx.BlendMode{},
			o.opacity,
			curve.Identity,
			curve.NewRectFromPoints(curve.Pt(-1e9, -1e9), curve.Pt(1e9, 1e9)).Path(0.1),
		)
		defer scene.PopLayer()
		p.PaintAt(o.Child, scene, curve.Point{})
	}
}

func (o *Opacity) SetOpacity(f float32) {
	if o.opacity != f {
		o.opacity = f
		MarkNeedsPaint(o)
	}
}

// TODO(dh): implement hit testing
type FittedBox struct {
	Box
	SingleChild

	fit  BoxFit
	clip bool
}

func (b *FittedBox) SetFit(f BoxFit) {
	if b.fit != f {
		b.fit = f
		MarkNeedsPaint(b)
	}
}

func (b *FittedBox) SetClip(clip bool) {
	if b.clip != clip {
		b.clip = clip
		MarkNeedsPaint(b)
	}
}

// PerformLayout implements Object.
func (b *FittedBox) PerformLayout() (size curve.Size) {
	if b.Child != nil {
		childSize := Layout(b.Child, Constraints{Max: curve.Sz(math.Inf(1), math.Inf(1))}, true)
		if b.fit == BoxFitScaleDown {
			cs := b.Constraints()
			cs.Min = curve.Size{}
			usz := cs.ConstrainWithAspectRatio(childSize)
			return b.Constraints().Constrain(usz)
		} else {
			return b.Constraints().ConstrainWithAspectRatio(childSize)
		}
	} else {
		return b.Constraints().Min
	}
}

// PerformPaint implements Object.
func (b *FittedBox) PerformPaint(p *Painter, scene *jello.Scene) {
	if b.Child == nil || b.Size() == curve.Sz(0, 0) || b.Child.Handle().Size() == curve.Sz(0, 0) {
		return
	}

	childSize := b.Child.Handle().Size()
	sizes := applyBoxFit(b.fit, childSize, b.Size())
	scaleX := sizes.Destination.Width / sizes.Source.Width
	scaleY := sizes.Destination.Height / sizes.Source.Height
	// TODO(dh): support alignment
	sourceRect := curve.NewRectFromOrigin(curve.Pt(0, 0), sizes.Source)
	destinationRect := curve.NewRectFromOrigin(curve.Pt(0, 0), sizes.Destination)
	hasVisualOverflow := sourceRect.Width() < childSize.Width || sourceRect.Height() < childSize.Height
	debug.Assert(!math.IsInf(scaleX, 0) && !math.IsInf(scaleY, 0))
	// TODO(dh): support alignment
	transform := curve.Scale(scaleX, scaleY)

	if hasVisualOverflow && b.clip {
		scene.PushLayer(
			gfx.BlendMode{Mix: gfx.MixClip},
			1,
			curve.Identity,
			destinationRect.Path(0.1),
		)
		defer scene.PopLayer()
	}
	childScene := p.Paint(b.Child)
	scene.Append(childScene, transform)
}

type fittedSizes struct {
	Source      curve.Size
	Destination curve.Size
}

func applyBoxFit(fit BoxFit, inputSize, outputSize curve.Size) fittedSizes {
	if inputSize.Height <= 0.0 || inputSize.Width <= 0.0 || outputSize.Height <= 0.0 || outputSize.Width <= 0.0 {
		return fittedSizes{}
	}

	var sourceSize, destinationSize curve.Size
	switch fit {
	case BoxFitFill:
		sourceSize = inputSize
		destinationSize = outputSize
	case BoxFitContain:
		sourceSize = inputSize
		if outputSize.Width/outputSize.Height > sourceSize.Width/sourceSize.Height {
			destinationSize = curve.Sz(sourceSize.Width*outputSize.Height/sourceSize.Height, outputSize.Height)
		} else {
			destinationSize = curve.Sz(outputSize.Width, sourceSize.Height*outputSize.Width/sourceSize.Width)
		}
	case BoxFitCover:
		if outputSize.Width/outputSize.Height > inputSize.Width/inputSize.Height {
			sourceSize = curve.Sz(inputSize.Width, inputSize.Width*outputSize.Height/outputSize.Width)
		} else {
			sourceSize = curve.Sz(inputSize.Height*outputSize.Width/outputSize.Height, inputSize.Height)
		}
		destinationSize = outputSize
	case BoxFitFitWidth:
		if outputSize.Width/outputSize.Height > inputSize.Width/inputSize.Height {
			// Like "cover"
			sourceSize = curve.Sz(inputSize.Width, inputSize.Width*outputSize.Height/outputSize.Width)
			destinationSize = outputSize
		} else {
			// Like "contain"
			sourceSize = inputSize
			destinationSize = curve.Sz(outputSize.Width, sourceSize.Height*outputSize.Width/sourceSize.Width)
		}
	case BoxFitFitHeight:
		if outputSize.Width/outputSize.Height > inputSize.Width/inputSize.Height {
			// Like "contain"
			sourceSize = inputSize
			destinationSize = curve.Sz(sourceSize.Width*outputSize.Height/sourceSize.Height, outputSize.Height)
		} else {
			// Like "cover"
			sourceSize = curve.Sz(inputSize.Height*outputSize.Width/outputSize.Height, inputSize.Height)
			destinationSize = outputSize
		}
	case BoxFitNone:
		sourceSize = curve.Sz(min(inputSize.Width, outputSize.Width), min(inputSize.Height, outputSize.Height))
		destinationSize = sourceSize
	case BoxFitScaleDown:
		sourceSize = inputSize
		destinationSize = inputSize
		aspectRatio := inputSize.Width / inputSize.Height
		if destinationSize.Height > outputSize.Height {
			destinationSize = curve.Sz(outputSize.Height*aspectRatio, outputSize.Height)
		}
		if destinationSize.Width > outputSize.Width {
			destinationSize = curve.Sz(outputSize.Width, outputSize.Width/aspectRatio)
		}
	}
	return fittedSizes{sourceSize, destinationSize}
}

type Lottie struct {
	Box

	composition *lottie_model.Composition
	frame       float64
}

func (l *Lottie) SetComposition(c *lottie_model.Composition) {
	if l.composition != c {
		l.composition = c
		MarkNeedsLayout(l)
		MarkNeedsPaint(l)
	}
}

func (l *Lottie) SetFrame(f float64) {
	if l.frame != f {
		l.frame = f
		MarkNeedsPaint(l)
	}
}

func (l *Lottie) PerformLayout() curve.Size {
	if l.composition != nil {
		w := float64(l.composition.Width)
		h := float64(l.composition.Height)
		return l.Constraints().Constrain(curve.Sz(w, h))
	} else {
		return l.Constraints().Constrain(curve.Sz(0, 0))
	}
}

func (l *Lottie) PerformPaint(p *Painter, scene *jello.Scene) {
	if l.composition == nil {
		return
	}
	var r lottie_renderer.Renderer
	r.Append(l.composition, l.frame, curve.Identity, 1, scene)
}

var _ Attacher = (*AnimatedOpacity)(nil)

// TODO(dh): why is AnimatedOpacity handled in the render layer when no other
// animated widget has such a special case? Having the render object listen to
// the animation is more efficient than having to rebuild the widget tree, but
// why don't other animations deserve this optimization?
type AnimatedOpacity struct {
	Box
	SingleChild

	opacity    animation.Animation[float32]
	oldOpacity float32

	updateOpacityListener base.Listener
}

// PerformLayout implements Object.
func (o *AnimatedOpacity) PerformLayout() (size curve.Size) {
	if o.Child != nil {
		return Layout(o.Child, o.constraints, true)
	} else {
		return o.constraints.Constrain(curve.Sz(0, 0))
	}
}

// PerformPaint implements Object.
func (o *AnimatedOpacity) PerformPaint(p *Painter, scene *jello.Scene) {
	alpha := o.opacity.Value()
	switch alpha {
	case 0:
		return
	case 1:
		p.PaintAt(o.Child, scene, curve.Point{})
	default:
		scene.PushLayer(
			gfx.BlendMode{},
			alpha,
			curve.Identity,
			curve.NewRectFromPoints(curve.Pt(-1e9, -1e9), curve.Pt(1e9, 1e9)).Path(0.1),
		)
		defer scene.PopLayer()
		p.PaintAt(o.Child, scene, curve.Point{})
	}
}

// PerformAttach implements Attacher.
func (o *AnimatedOpacity) PerformAttach(r *Renderer) {
	// TODO(dh): we'd prefer AfterAttach and BeforeDetach hooks instead of
	// PerformAttach, so that we don't have to concern ourselves with attaching
	// children
	for child := range o.Children() {
		Attach(child, r)
	}
	o.updateOpacityListener = o.opacity.AddListener(o.updateOpacity)
	o.updateOpacity()
}

// PerformDetach implements Attacher.
func (o *AnimatedOpacity) PerformDetach() {
	o.opacity.RemoveListener(o.updateOpacityListener)
}

func (o *AnimatedOpacity) SetOpacity(anim animation.Animation[float32]) {
	if o.opacity == anim {
		return
	}
	if o.Attached() && o.opacity != nil {
		o.opacity.RemoveListener(o.updateOpacityListener)
	}
	o.opacity = anim
	if o.Attached() {
		o.updateOpacityListener = o.opacity.AddListener(o.updateOpacity)
	}
	o.updateOpacity()
}

func (o *AnimatedOpacity) updateOpacity() {
	if o.oldOpacity != o.opacity.Value() {
		o.oldOpacity = o.opacity.Value()
		MarkNeedsPaint(o)
	}
}
