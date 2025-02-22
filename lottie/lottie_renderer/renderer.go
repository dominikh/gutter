// SPDX-FileCopyrightText: 2024 the Velato Authors
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_renderer

import (
	"fmt"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	model "honnef.co/go/gutter/lottie/lottie_model"
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/jello"
	"honnef.co/go/jello/gfx"
)

type Renderer struct {
	batch        batch
	maskElements curve.BezPath
	fullRect     curve.BezPath
}

func (r *Renderer) Render(
	animation *model.Composition,
	frame float64,
	transform curve.Affine,
	alpha float64,
) jello.Scene {
	var scene jello.Scene
	r.Append(animation, frame, transform, alpha, &scene)
	return scene
}

func (r *Renderer) Append(
	anim *model.Composition,
	frame float64,
	trans curve.Affine,
	alpha float64,
	scene *jello.Scene,
) {
	r.fullRect = curve.Rect{
		X0: 0,
		Y0: 0,
		X1: float64(anim.Width),
		Y1: float64(anim.Height),
	}.Path(0)
	r.batch.reset(r)

	mix := gfx.MixClip
	if alpha != 0 {
		mix = gfx.MixNormal
	}
	scene.PushLayer(gfx.BlendMode{Mix: mix}, float32(alpha), trans, curve.Rect{
		X0: 0,
		Y0: 0,
		X1: float64(anim.Width),
		Y1: float64(anim.Height),
	}.Path(0.1))
	defer scene.PopLayer()

	for i := len(anim.Layers) - 1; i >= 0; i-- {
		layer := anim.Layers[i]
		if layer.IsMask {
			continue
		}
		r.renderLayer(
			anim,
			anim.Layers,
			layer,
			trans,
			frame,
			scene,
		)
	}
}

func (r *Renderer) renderLayer(
	anim *model.Composition,
	layerSet []model.Layer,
	layer model.Layer,
	trans curve.Affine,
	frame float64,
	scene *jello.Scene,
) {
	if frame < layer.FirstFrame || frame >= layer.LastFrame {
		return
	}

	switch alpha := layer.Opacity.Evaluate(frame) / 100.0; alpha {
	case 0:
		return
	case 1:
		scene.PushLayer(gfx.BlendMode{Mix: gfx.MixClip}, 1, curve.Identity, r.fullRect)
		defer scene.PopLayer()
	default:
		scene.PushLayer(gfx.BlendMode{}, float32(alpha), curve.Identity, r.fullRect)
		defer scene.PopLayer()
	}

	parentTransform := trans
	trans = r.computeTransform(layerSet, layer, parentTransform, frame)
	if maskIndex, ok := layer.MaskLayerID.Get(); ok {
		if mode, ok := layer.MaskLayerMode.Get(); ok {
			scene.PushLayer(
				gfx.BlendMode{},
				1.0,
				parentTransform,
				r.fullRect,
			)
			defer scene.PopLayer()
			if maskIndex >= 0 && maskIndex < len(layerSet) {
				r.renderLayer(
					anim,
					layerSet,
					layerSet[maskIndex],
					parentTransform,
					frame,
					scene,
				)
			}

			scene.PushLayer(mode, 1.0, parentTransform, r.fullRect)
			defer scene.PopLayer()
		}
	}
	for _, mask := range layer.Masks {
		alpha := mask.Opacity.Evaluate(frame) / 100.0
		r.maskElements = mask.Geometry.Evaluate(frame, r.maskElements)
		mode := gfx.MixClip
		if alpha != 1 {
			mode = gfx.MixNormal
		}
		scene.PushLayer(
			gfx.BlendMode{Mix: mode},
			float32(alpha),
			trans,
			r.maskElements,
		)
		r.maskElements = r.maskElements[:0]
	}
	switch layer.Content.Kind {
	case model.ContentKindNone:
	case model.ContentKindInstance:
		if assetLayers, ok := anim.Assets[layer.Content.Instance.Name]; ok {
			if tm, ok := layer.Content.Instance.TimeRemap.Get(); ok {
				// Time remapping maps frame to time in seconds. That means our
				// 'frame+frameDelta' is really time 's' in the precomposition.
				//
				// In this mode, time stretch and start time are ignored. That's
				// not documented anywhere we could find, but matches the
				// behavior of lottie-web and Skottie.
				s := tm.Evaluate(frame)
				// Map time to frame
				frame = s * anim.Framerate
			} else {
				frame = (frame - layer.StartFrame) / layer.Stretch
			}

			scene.PushLayer(gfx.BlendMode{Mix: gfx.MixNormal}, 1, trans, curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(layer.Width, layer.Height)).Path(0.1))
			defer scene.PopLayer()

			for i := len(assetLayers) - 1; i >= 0; i-- {
				assetLayer := assetLayers[i]
				if assetLayer.IsMask {
					continue
				}
				r.renderLayer(
					anim,
					assetLayers,
					assetLayer,
					trans,
					frame,
					scene,
				)
			}
		}
	case model.ContentKindShapes:
		r.renderShapes(scene, layer.Content.Shapes, trans, frame)
		r.batch.render(scene, &r.batch.draws[0])
		r.batch.reset(r)
	}

	n := len(layer.Masks)
	for range n {
		scene.PopLayer()
	}
}

func (r *Renderer) renderShapes(
	scene *jello.Scene,
	shapes []model.Shape,
	trans curve.Affine,
	frame float64,
) {
	// Keep track of our local top of the geometry stack. Any subsequent
	// draws are bounded by this.
	geometryStart := len(r.batch.geometries)
	// Also keep track of top of draw stack for repeater evaluation.
	drawStart := len(r.batch.draws[r.batch.curGroup].children)
	// Top to bottom, collect geometries and draws.
	for _, shape := range shapes {
		switch shape.Kind {
		case model.ShapeKindGroup:
			groupTransform := curve.Identity
			groupAlpha := 1.0
			if t, ok := shape.GroupTransform.Get(); ok {
				groupTransform = t.Transform.Evaluate(frame)
				groupAlpha = t.Opacity.Evaluate(frame) / 100.0
			}
			r.batch.pushGroup(groupAlpha)
			r.renderShapes(scene, shape.GroupShapes, trans.Mul(groupTransform), frame)
			r.batch.popGroup()
		case model.ShapeKindGeometry:
			r.batch.pushGeometry(&shape.Geometry, trans, frame)
		case model.ShapeKindDraw:
			r.batch.pushDraw(shape.Draw, geometryStart, frame)
		case model.ShapeKindRepeater:
			_ = drawStart
			panic("TODO")
			// repeater := shape.Repeater.Evaluate(frame)
			// r.batch.repeat(&repeater, geometryStart, drawStart)
		}
	}
}

// Computes the transform for a single layer. This currently chases the
// full transform chain each time. If it becomes a bottleneck, we can
// implement caching.
func (r *Renderer) computeTransform(
	layerSet []model.Layer,
	layer model.Layer,
	globalTransform curve.Affine,
	frame float64,
) curve.Affine {
	transform := layer.Transform.Evaluate(frame)
	parentIndex := layer.Parent
	count := 0
	for {
		index, ok := parentIndex.Get()
		if !ok {
			break
		}
		// We don't check for cycles at import time, so this heuristic
		// prevents infinite loops.
		if count >= len(layerSet) {
			break
		}
		if index >= 0 && index < len(layerSet) {
			parent := layerSet[index]
			parentIndex = parent.Parent
			transform = parent.Transform.Evaluate(frame).Mul(transform)
			count++
		} else {
			break
		}
	}
	return globalTransform.Mul(transform)
}

type drawDataKind int

const (
	drawDataDraw drawDataKind = iota
	drawDataGroup
)

type drawData struct {
	kind drawDataKind

	stroke   maybe.Option[curve.Stroke]
	brush    gfx.Brush
	alpha    float64
	geometry [2]int

	parent     int
	groupAlpha float64
	children   []int32
}

func newDrawData(draw model.Draw, geometry [2]int, frame float64) drawData {
	return drawData{
		kind: drawDataDraw,
		stroke: maybe.Map(
			draw.Stroke,
			func(stroke animation.KeyframedStroke) curve.Stroke { return stroke.Evaluate(frame) },
		),
		brush:    draw.Brush.Evaluate(1, frame),
		alpha:    draw.Opacity.Evaluate(frame) / 100.0,
		geometry: geometry,
	}
}

type geometryData struct {
	elements  [2]int
	transform curve.Affine
}

type batch struct {
	elements   curve.BezPath
	geometries []geometryData
	// repeatGeometries []GeometryData
	// repeatDraws      []DrawData
	// Length of geometries at time of most recent draw. This is used to prevent
	// merging into already used geometries.
	drawnGeometry int
	root          drawData
	fullRect      curve.BezPath
	draws         []drawData
	curGroup      int
}

func (b *batch) pushGroup(opacity float64) {
	child := drawData{
		kind:       drawDataGroup,
		parent:     b.curGroup,
		groupAlpha: opacity,
	}
	b.draws = append(b.draws, child)
	b.draws[b.curGroup].children = append(b.draws[b.curGroup].children, int32(len(b.draws)-1))
	b.curGroup = len(b.draws) - 1
}

func (b *batch) popGroup() {
	b.curGroup = b.draws[b.curGroup].parent
}

func (b *batch) pushGeometry(geometry *model.Geometry, transform curve.Affine, frame float64) {
	// Merge with the previous geometry if possible. There are two
	// conditions:
	// 1. The previous geometry has not yet been referenced by a draw
	// 2. The geometries have the same transform
	if b.drawnGeometry < len(b.geometries) && b.geometries[len(b.geometries)-1].transform == transform {
		b.elements = geometry.Evaluate(frame, b.elements)
		b.geometries[len(b.geometries)-1].elements[1] = len(b.elements)
	} else {
		start := len(b.elements)
		b.elements = geometry.Evaluate(frame, b.elements)
		end := len(b.elements)
		b.geometries = append(b.geometries, geometryData{
			elements:  [2]int{start, end},
			transform: transform,
		})
	}
}

func (b *batch) pushDraw(draw model.Draw, geometryStart int, frame float64) {
	b.draws = append(b.draws, newDrawData(draw, [2]int{geometryStart, len(b.geometries)}, frame))
	b.draws[b.curGroup].children = append(b.draws[b.curGroup].children, int32(len(b.draws)-1))
	b.drawnGeometry = len(b.geometries)
}

func (b *batch) repeat(repeater *model.Repeater, geometryStart int, drawStart int) {
	panic("TODO")
}

func (b *batch) render(scene *jello.Scene, group *drawData) {
	if group.kind != drawDataGroup {
		panic(fmt.Sprintf("internal error: wrong kind %v", group.kind))
	}

	if group.groupAlpha != 1 {
		scene.PushLayer(gfx.BlendMode{}, float32(group.groupAlpha), curve.Identity, b.fullRect)
		defer scene.PopLayer()
	}

	// Process all draws in reverse
	for i := len(group.children) - 1; i >= 0; i-- {
		draw := &b.draws[group.children[i]]
		switch draw.kind {
		case drawDataDraw:
			brush := model.BrushWithAlpha(draw.brush, draw.alpha)
			for _, geometry := range b.geometries[draw.geometry[0]:draw.geometry[1]] {
				path := b.elements[geometry.elements[0]:geometry.elements[1]]
				transform := geometry.transform
				if stroke, ok := draw.stroke.Get(); ok {
					// Skip zero-width strokes to work around
					// https://github.com/linebender/vello/issues/662
					if stroke.Width != 0 {
						scene.Stroke(stroke, transform, brush, curve.Identity, path)
					}
				} else {
					scene.Fill(gfx.NonZero, transform, brush, curve.Identity, path)
				}
			}
		case drawDataGroup:
			b.render(scene, draw)
		default:
			panic(fmt.Sprintf("internal error: unexpected draw data kind %v", draw.kind))
		}
	}
}

func (b *batch) reset(r *Renderer) {
	b.elements = b.elements[:0]
	b.geometries = b.geometries[:0]
	if cap(b.draws) == 0 {
		b.draws = make([]drawData, 1)
	} else {
		b.draws = b.draws[:1]
	}
	b.draws[0] = drawData{
		kind:       drawDataGroup,
		groupAlpha: 1,
	}
	b.curGroup = 0
	// b.repeatGeometries = b.repeatGeometries[:0]
	b.drawnGeometry = 0
	b.fullRect = r.fullRect
}
