// SPDX-FileCopyrightText: 2024 the Velato Authors
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_model

import (
	"fmt"
	"math"
	"slices"
	"time"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/stuff/container/maybe"
)

var (
	// Colors in Lottie files are in sRGB
	ParsedColorSpace = color.SRGB

	// Lottie files are authored for sRGB blending and gradients. We don't
	// currently support blending in a non-linear color space, but we can
	// interpolate gradients in any color space we want.
	WorkingColorSpace = color.SRGB
)

type Composition struct {
	FirstFrame float64
	LastFrame  float64
	Framerate  float64
	Width      int
	Height     int
	Assets     map[string][]Layer
	Layers     []Layer
}

func (c *Composition) Duration() time.Duration {
	return time.Duration(math.Round(((c.LastFrame - c.FirstFrame) / c.Framerate) * float64(time.Second)))
}

type GeometryKind int

const (
	GeometryKindFixed GeometryKind = iota + 1
	GeometryKindRect
	GeometryKindEllipse
	GeometryKindSpline
)

type Geometry struct {
	Kind    GeometryKind
	Fixed   []curve.PathElement
	Rect    animation.KeyframedRoundedRect
	Ellipse animation.KeyframedEllipse
	Spline  Spline
}

func (g Geometry) Evaluate(frame float64, path curve.BezPath) curve.BezPath {
	switch g.Kind {
	case GeometryKindFixed:
		return append(path, g.Fixed...)
	case GeometryKindRect:
		return slices.AppendSeq(path, g.Rect.Evaluate(frame).PathElements(0.1))
	case GeometryKindEllipse:
		return slices.AppendSeq(path, g.Ellipse.Evaluate(frame).PathElements(0.1))
	case GeometryKindSpline:
		path, _ = g.Spline.Evaluate(frame, path)
		return path
	default:
		panic(fmt.Sprintf("internal error: unhandled geometry kind %v", g.Kind))
	}
}

type Draw struct {
	Stroke maybe.Option[animation.KeyframedStroke]
	Brush  Brush
	// XXX use 0-1, not 0-100
	Opacity animation.Keyframes[float64]
}

type ShapeKind int

const (
	ShapeKindGroup ShapeKind = iota + 1
	ShapeKindGeometry
	ShapeKindDraw
	ShapeKindRepeater
)

// OPT(dh): don't use a fat union for this
type Shape struct {
	Kind           ShapeKind
	GroupShapes    []Shape
	GroupTransform maybe.Option[GroupTransform]
	Geometry       Geometry
	Draw           Draw
	Repeater       Repeater
}

type GroupTransform struct {
	Transform animation.KeyframedTransform
	// XXX use 0-1, not 0-100
	Opacity animation.Keyframes[float64]
}

type Layer struct {
	Name      string
	Parent    maybe.Option[int]
	Transform animation.KeyframedTransform
	// XXX use 0-1, not 0-100
	Opacity    animation.Keyframes[float64]
	Width      float64
	Height     float64
	BlendMode  maybe.Option[gfx.BlendMode]
	FirstFrame float64
	LastFrame  float64
	Stretch    float64
	StartFrame float64
	Masks      []Mask
	IsMask     bool
	// TODO(dh): the two Mask fields should be a single field
	MaskLayerMode maybe.Option[gfx.BlendMode]
	MaskLayerID   maybe.Option[int]
	Content       Content
}

type Mask struct {
	Mode     gfx.BlendMode
	Geometry Geometry
	// XXX use 0-1, not 0-100
	Opacity animation.Keyframes[float64]
}

type ContentKind int

const (
	ContentKindNone ContentKind = iota
	ContentKindInstance
	ContentKindShapes
)

type Content struct {
	Kind     ContentKind
	Instance struct {
		Name      string
		TimeRemap maybe.Option[animation.Keyframes[float64]]
	}
	Shapes []Shape
}
