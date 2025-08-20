// SPDX-FileCopyrightText: 2024 the Velato Authors
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_converter

import (
	"fmt"
	"math"
	"slices"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/gfx"
	encoding "honnef.co/go/gutter/lottie/lottie_encoding"
	model "honnef.co/go/gutter/lottie/lottie_model"
	"honnef.co/go/stuff/container/maybe"
	"honnef.co/go/stuff/math/mathutil"
)

func ConvertAnimation(source *encoding.Animation) *model.Composition {
	target := &model.Composition{
		FirstFrame: source.InPoint,
		LastFrame:  source.OutPoint,
		Framerate:  source.Framerate,
		Width:      source.Width,
		Height:     source.Height,
		Assets:     make(map[string][]model.Layer),
	}

	// Collect assets and layers
	idmap := map[int]int{}
	for _, asset := range source.Assets {
		switch asset := asset.(type) {
		case encoding.Precomposition:
			clear(idmap)
			var layers []model.Layer
			var maskLayer maybe.Option[int]
			for _, layer := range asset.Layers {
				idx := len(layers)
				if layer, id, maskBlend, ok := convertLayer(layer); ok {
					if maskBlend, ok := maskBlend.Get(); ok {
						if maskLayer, ok := maskLayer.Take().Get(); ok {
							layer.MaskLayerMode = maybe.Some(maskBlend)
							layer.MaskLayerID = maybe.Some(maskLayer)
						}
					}
					if layer.IsMask {
						maskLayer = maybe.Some(idx)
					}
					idmap[id] = idx
					layers = append(layers, layer)
				}
			}
			for i := range layers {
				layer := &layers[i]
				if parent, ok := layer.Parent.Get(); ok {
					layer.Parent = maybe.Some(idmap[parent])
				}
			}
			target.Assets[asset.ID] = layers
		default:
			panic(fmt.Sprintf("asset type %T not yet supported", asset))
		}
	}

	clear(idmap)
	var layers []model.Layer
	var maskLayer maybe.Option[int]
	for _, layer := range source.Layers {
		idx := len(layers)
		if layer, id, maskBlend, ok := convertLayer(layer); ok {
			if maskBlend, ok := maskBlend.Get(); ok {
				if maskLayer, ok := maskLayer.Take().Get(); ok {
					layer.MaskLayerID = maybe.Some(maskLayer)
					layer.MaskLayerMode = maybe.Some(maskBlend)
				}
			}
			if layer.IsMask {
				maskLayer = maybe.Some(idx)
			}
			idmap[id] = idx
			layers = append(layers, layer)
		}
	}
	for i := range layers {
		layer := &layers[i]
		if parent, ok := layer.Parent.Get(); ok {
			layer.Parent = maybe.Some(idmap[parent])
		}
	}
	target.Layers = layers
	return target
}

func convertLayer(source encoding.AnyLayer) (model.Layer, int, maybe.Option[gfx.BlendMode], bool) {
	var layer model.Layer
	var none maybe.Option[gfx.BlendMode]

	var id int
	var matteMode maybe.Option[gfx.BlendMode]
	switch l := source.(type) {
	case encoding.NullLayer:
		if l.Hidden {
			return model.Layer{}, 0, none, false
		}
		id, matteMode = setupLayerBase(l.VisualLayer, &layer)

	case encoding.PrecompositionLayer:
		if l.Hidden {
			return model.Layer{}, 0, none, false
		}
		id, matteMode = setupPrecompLayer(l, &layer)
		name := l.ReferenceID
		timeRemap := maybe.Map(l.TimeRemap, convertScalar)
		layer.Content = model.Content{
			Kind: model.ContentKindInstance,
			Instance: struct {
				Name      string
				TimeRemap maybe.Option[animation.Keyframes[float64]]
			}{name, timeRemap},
		}

	case encoding.ShapeLayer:
		if l.Hidden {
			return model.Layer{}, 0, none, false
		}
		id, matteMode = setupShapeLayer(l, &layer)
		var shapes []model.Shape
		for _, shape := range l.Shapes {
			if shape, ok := convertShape(shape); ok {
				shapes = append(shapes, shape)
			}
		}
		layer.Content = model.Content{
			Kind:   model.ContentKindShapes,
			Shapes: shapes,
		}

	case encoding.SolidLayer:
		if l.Hidden {
			return model.Layer{}, 0, none, false
		}
		id, matteMode = setupLayerBase(l.VisualLayer, &layer)
	}

	return layer, id, matteMode, true
}

func setupLayerBase(source encoding.VisualLayer, target *model.Layer) (int, maybe.Option[gfx.BlendMode]) {
	target.Name = source.Name
	target.Parent = source.ParentIndex
	transform, opacity := convertTransform(&source.Transform)
	target.Transform = transform
	target.Opacity = opacity
	target.IsMask = bool(source.MatteTarget)

	matteMode := maybe.Map(source.MatteMode, func(v encoding.MatteMode) gfx.BlendMode {
		switch v {
		case encoding.MatteModeNormal:
			return gfx.BlendMode{
				Mix: gfx.MixNormal,
			}
		case encoding.MatteModeAlpha, encoding.MatteModeLuma:
			return gfx.BlendMode{
				Compose: gfx.ComposeSrcIn,
			}
		case encoding.MatteModeInvertedAlpha, encoding.MatteModeInvertedLuma:
			return gfx.BlendMode{
				Compose: gfx.ComposeSrcOut,
			}
		default:
			return gfx.BlendMode{}
		}
	})

	target.BlendMode = convertBlendMode(source.BlendMode)
	// TODO: Why do we do this next part?
	if target.BlendMode == maybe.Some(gfx.BlendMode{}) {
		target.BlendMode = maybe.Option[gfx.BlendMode]{}
	}
	target.Stretch = source.TimeStretch.UnwrapOr(1)
	target.FirstFrame = source.InPoint
	target.LastFrame = source.OutPoint
	target.StartFrame = source.StartTime

	for _, maskSource := range source.MasksProperties {
		// TODO(dh): what does a mask without a shape do?
		if shape, ok := maskSource.Shape.Get(); ok {
			if geometry, ok := convertShapeGeometry(shape); ok {
				// TODO(dh): correctly implement masks. Compose together
				// multiple masks into a single mask, and propagate normal
				// layer's blend mode through the final mask layer, so that its
				// isolated blend group doesn't cause problems.
				//
				// We could either draw actual mask layers, filling the
				// geometry, then compose the layers together. However, it'd be
				// preferable to do boolean math directly on the geometry and
				// generate a clip instead of a mask layer. That way we can
				// avoid a lot blending. Not all mask modes can be implemented
				// that way (lighten, darken, possibly difference), but we can
				// use both boolean math and fills on a single stack of masks.
				var mode gfx.BlendMode
				switch maskSource.Mode {
				case encoding.MaskModeNone:
				case encoding.MaskModeAdd:
				case encoding.MaskModeSubtract:
				case encoding.MaskModeIntersect:
				case encoding.MaskModeLighten:
				case encoding.MaskModeDarken:
				case encoding.MaskModeDifference:
				default:
				}
				oneHundred := encoding.ScalarProperty{
					AnimatableProperty: encoding.AnimatableProperty[float64, encoding.SimpleKeyframe[[]float64]]{
						Value: 100,
					},
				}
				opacity := convertScalar(maskSource.Opacity.UnwrapOr(oneHundred))
				target.Masks = append(target.Masks, model.Mask{
					Mode:     mode,
					Geometry: geometry,
					Opacity:  opacity,
				})
			}
		}
	}

	return source.Index, matteMode
}

func setupPrecompLayer(source encoding.PrecompositionLayer, target *model.Layer) (int, maybe.Option[gfx.BlendMode]) {
	target.Width = source.Width
	target.Height = source.Height
	return setupLayerBase(source.VisualLayer, target)
}

func convertBlendMode(value encoding.BlendMode) maybe.Option[gfx.BlendMode] {
	switch value {
	case encoding.BlendModeNormal:
		return maybe.Option[gfx.BlendMode]{}
	case encoding.BlendModeMultiply:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixMultiply})
	case encoding.BlendModeScreen:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixScreen})
	case encoding.BlendModeOverlay:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixOverlay})
	case encoding.BlendModeDarken:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixDarken})
	case encoding.BlendModeLighten:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixLighten})
	case encoding.BlendModeColorDodge:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixColorDodge})
	case encoding.BlendModeColorBurn:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixColorBurn})
	case encoding.BlendModeHardLight:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixHardLight})
	case encoding.BlendModeSoftLight:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixSoftLight})
	case encoding.BlendModeDifference:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixDifference})
	case encoding.BlendModeExclusion:
		return maybe.Some(gfx.BlendMode{Mix: gfx.MixExclusion})
	case encoding.BlendModeHue:
		// return maybe.Some(gfx.BlendMode{Mix: gfx.MixHue})
		// XXX add support
		panic("unimplemented")
	case encoding.BlendModeSaturation:
		// return maybe.Some(gfx.BlendMode{Mix: gfx.MixSaturation})
		// XXX add support
		panic("unimplemented")
	case encoding.BlendModeColor:
		// return maybe.Some(gfx.BlendMode{Mix: gfx.MixColor})
		// XXX add support
		panic("unimplemented")
	case encoding.BlendModeLuminosity:
		// return maybe.Some(gfx.BlendMode{Mix: gfx.MixLuminosity})
		// XXX add support
		panic("unimplemented")
	case encoding.BlendModeAdd:
		// XXX add support
		panic("unimplemented")
	case encoding.BlendModeHardMix:
		// XXX add support
		panic("unimplemented")
	default:
		panic("unimplemented")
		return maybe.Option[gfx.BlendMode]{}
	}
}

func convertTransform(value *encoding.Transform) (animation.KeyframedTransform, animation.Keyframes[float64]) {
	var position animation.KeyframedPoint

	if value.Position.Split {
		position = animation.KeyframedPoint{
			X: convertScalar(value.Position.X),
			Y: convertScalar(value.Position.Y),
		}
	} else {
		position = convertPos(value.Position.PositionProperty)
	}

	one := encoding.VectorProperty{
		AnimatableProperty: encoding.AnimatableProperty[encoding.Vec2, encoding.SimpleKeyframe[encoding.Vec2]]{
			Value: encoding.Vec2{1, 1},
		},
	}
	transform := animation.KeyframedTransform{
		Anchor:    convertPos(value.AnchorPoint),
		Position:  position,
		Scale:     convertVec2(value.Scale.UnwrapOr(one)),
		Rotation:  convertScalar(value.Rotation),
		Skew:      convertScalar(value.Skew),
		SkewAngle: convertScalar(value.SkewAxis),
	}
	for i, v := range transform.Rotation.Values {
		transform.Rotation.Values[i] = toRadians(v)
	}
	for i, v := range transform.Skew.Values {
		v = mathutil.Clamp(-v, -85, 85)
		transform.Skew.Values[i] = toRadians(v)
	}
	for i, v := range transform.SkewAngle.Values {
		transform.SkewAngle.Values[i] = toRadians(v)
	}

	oneHundred := encoding.ScalarProperty{
		AnimatableProperty: encoding.AnimatableProperty[float64, encoding.SimpleKeyframe[[]float64]]{
			Value: 100,
		},
	}
	opacity := convertScalar(value.Opacity.UnwrapOr(oneHundred))

	return transform, opacity
}

func convertScalar(floatValue encoding.ScalarProperty) animation.Keyframes[float64] {
	if floatValue.Animated {
		n := len(floatValue.Keyframes)
		frames := make([]float64, n)
		easings := make([]animation.Curve, n)
		values := make([]float64, n)

		for i, keyframe := range floatValue.Keyframes {
			frames[i], easings[i] = convertKeyframe(keyframe.BaseKeyframe)
			values[i] = keyframe.Value[0]
		}
		return animation.Keyframes[float64]{
			Frames: frames,
			Curves: easings,
			Values: values,
		}
	} else {
		return fixedValue(floatValue.Value)
	}
}

func convertKeyframeHandle(handle encoding.EasingHandle) curve.Point {
	// XXX validate length
	x := handle.X[0]
	y := handle.Y[0]
	return curve.Point{X: x, Y: y}
}

func convertMultiKeyframes[T ~[]float64 | ~[2]float64 | ~[3]float64](keyframes []encoding.SimpleKeyframe[T], numItems int) []animation.Keyframes[float64] {
	collectTangents := func(handle_ maybe.Option[encoding.EasingHandle]) []curve.Point {
		handle, ok := handle_.Get()
		if !ok {
			return nil
		}
		handles := make([]curve.Point, numItems)
		if len(handle.X) == 0 || len(handle.Y) == 0 {
			// Having no values is not valid
			return handles
		}
		for i := range numItems {
			var x, y float64
			if i < len(handle.X) {
				x = handle.X[i]
			} else {
				x = handle.X[len(handle.X)-1]
			}
			if i < len(handle.Y) {
				y = handle.Y[i]
			} else {
				y = handle.Y[len(handle.Y)-1]
			}
			handles[i] = curve.Point{X: x, Y: y}
		}
		return handles
	}

	// OPT(dh): preallocate
	// value per keyframe per dimension
	frames := make([][]float64, numItems)
	easings := make([][]animation.Curve, numItems)
	values := make([][]float64, numItems)

	for _, keyframe := range keyframes {
		inTangents := collectTangents(keyframe.InTangent)
		outTangents := collectTangents(keyframe.OutTangent)
		if len(inTangents) == 0 || len(outTangents) == 0 {
			for j := range numItems {
				frames[j] = append(frames[j], keyframe.Time)
				if keyframe.Hold {
					easings[j] = append(easings[j], animation.CurveStatic(0))
				} else {
					easings[j] = append(easings[j], animation.CurveIdentity)
				}
				values[j] = append(values[j], keyframe.Value[j])
			}
		} else {
			for j := range numItems {
				inTangent := inTangents[j]
				outTangent := outTangents[j]
				frames[j] = append(frames[j], keyframe.Time)
				if keyframe.Hold {
					easings[j] = append(easings[j], animation.CurveStatic(0))
				} else {
					easings[j] = append(easings[j], animation.CurveCubicBezier{
						P2: inTangent,
						P1: outTangent,
					})
				}
				if j < len(keyframe.Value) {
					values[j] = append(values[j], keyframe.Value[j])
				} else {
					values[j] = append(values[j], 0)
				}
			}
		}
	}
	out := make([]animation.Keyframes[float64], numItems)
	for i := range out {
		out[i] = animation.Keyframes[float64]{
			Frames: frames[i],
			Curves: easings[i],
			Values: values[i],
		}
	}
	return out
}

func convertPos(pos encoding.PositionProperty) animation.KeyframedPoint {
	if pos.Animated {
		// TODO: Are we using PositionKeyframes here how we're supposed to?
		// there are in_tangents and out_tangents in addition to the keyframes.
		// conv_keyframes(pos_keyframes.iter().map(|pk| &pk.keyframe), |k| f(&k.value))
		xy := convertMultiKeyframes(pos.Keyframes, 2)
		return animation.KeyframedPoint{
			X: xy[0],
			Y: xy[1],
		}
	} else {
		return animation.KeyframedPoint{
			X: fixedValue(pos.Value[0]),
			Y: fixedValue(pos.Value[1]),
		}
	}
}

func convert2D(v encoding.VectorProperty) []animation.Keyframes[float64] {
	if v.Animated {
		return convertMultiKeyframes(v.Keyframes, 2)
	} else {
		return []animation.Keyframes[float64]{
			fixedValue(v.Value[0]),
			fixedValue(v.Value[1]),
		}
	}
}

func convertVec2(value encoding.VectorProperty) animation.KeyframedVec2 {
	xy := convert2D(value)
	return animation.KeyframedVec2{
		X: xy[0],
		Y: xy[1],
	}
}

func convertSize(value encoding.VectorProperty) animation.KeyframedSize {
	wh := convert2D(value)
	return animation.KeyframedSize{
		Width:  wh[0],
		Height: wh[1],
	}
}

func convertPoint(value encoding.PositionProperty) animation.KeyframedPoint {
	xy := convert2D(value)
	return animation.KeyframedPoint{
		X: xy[0],
		Y: xy[1],
	}
}

func convertShapeGeometry(value encoding.BezierProperty) (model.Geometry, bool) {
	if value.Animated {
		var isClosed bool
		n := len(value.Keyframes)
		frames := make([]float64, n)
		easings := make([]animation.Curve, n)
		values := make([][]curve.Point, n)
		for i, value := range value.Keyframes {
			if len(value.Value) == 0 {
				return model.Geometry{}, false
			}
			frames[i], easings[i] = convertKeyframe(value.BaseKeyframe)
			points, isFrameClosed := convertSpline(value.Value[0])
			values[i] = points
			if isFrameClosed {
				isClosed = true
			}
		}
		return model.Geometry{
			Kind: model.GeometryKindSpline,
			Spline: model.Spline{
				IsClosed: isClosed,
				Keyframes: animation.Keyframes[[]curve.Point]{
					Frames: frames,
					Curves: easings,
					Values: values,
				},
			},
		}, true
	} else {
		points, isClosed := convertSpline(value.Value)
		path, _ := model.ToPath(points, isClosed, nil)
		return model.Geometry{
			Kind:  model.GeometryKindFixed,
			Fixed: path,
		}, true
	}
}

func convertSpline(value encoding.Bezier) ([]curve.Point, bool) {
	points := make([]curve.Point, 0, len(value.Vertices)*3)
	isClosed := value.Closed
	for i := range max(len(value.Vertices), len(value.InTangents), len(value.OutTangents)) {
		var v, in, out [2]float64
		if i < len(value.Vertices) {
			v = value.Vertices[i]
		}
		if i < len(value.InTangents) {
			in = value.InTangents[i]
		}
		if i < len(value.OutTangents) {
			out = value.OutTangents[i]
		}

		points = append(points,
			curve.Pt(v[0], v[1]),
			curve.Pt(in[0], in[1]),
			curve.Pt(out[0], out[1]),
		)
	}
	return points, isClosed
}

func convertShape(value encoding.AnyGraphicElement) (model.Shape, bool) {
	if draw, ok := convertDraw(value); ok {
		return model.Shape{
			Kind: model.ShapeKindDraw,
			Draw: draw,
		}, true
	} else if geometry, ok := convertGeometry(value); ok {
		return model.Shape{
			Kind:     model.ShapeKindGeometry,
			Geometry: geometry,
		}, true
	}

	switch value := value.(type) {
	case encoding.Group:
		var shapes []model.Shape
		var groupTransform maybe.Option[model.GroupTransform]
		for _, item := range value.Shapes {
			switch item := item.(type) {
			case encoding.TransformShape:
				groupTransform = maybe.Some(convertShapeTransform(item))
			default:
				if shape, ok := convertShape(item); ok {
					shapes = append(shapes, shape)
				}
			}
		}
		if len(shapes) != 0 {
			return model.Shape{
				Kind:           model.ShapeKindGroup,
				GroupShapes:    shapes,
				GroupTransform: groupTransform,
			}, true
		} else {
			return model.Shape{}, false
		}
	default:
		// TODO: support repeater shape
		return model.Shape{}, false
	}
}

func convertGeometry(value encoding.AnyGraphicElement) (model.Geometry, bool) {
	switch value := value.(type) {
	case encoding.Ellipse:
		return model.Geometry{
			Kind: model.GeometryKindEllipse,
			Ellipse: animation.KeyframedEllipse{
				Position: convertPos(value.Position),
				Size:     convertSize(value.Size),
			},
		}, true
	case encoding.Rectangle:
		return model.Geometry{
			Kind: model.GeometryKindRect,
			Rect: animation.KeyframedRoundedRect{
				Position:     convertPos(value.Position),
				Size:         convertSize(value.Size),
				CornerRadius: convertScalar(value.Rounded),
			},
		}, true
	case encoding.Path:
		return convertShapeGeometry(value.Shape)
	case encoding.Polystar:
		// TODO support shape
		return model.Geometry{}, false
	default:
		return model.Geometry{}, false
	}
}

func convertShapeTransform(value encoding.TransformShape) model.GroupTransform {
	transform, opacity := convertTransform(&value.Transform)
	return model.GroupTransform{
		Transform: transform,
		Opacity:   opacity,
	}
}

func convertDraw(value encoding.AnyGraphicElement) (model.Draw, bool) {
	if value, ok := value.(interface{ IsHidden() bool }); ok && value.IsHidden() {
		return model.Draw{}, false
	}
	oneHundred := encoding.ScalarProperty{
		AnimatableProperty: encoding.AnimatableProperty[float64, encoding.SimpleKeyframe[[]float64]]{
			Value: 100,
		},
	}
	switch value := value.(type) {
	case encoding.Fill:
		color := convertColor(value.Color)
		brush := model.Brush{
			Kind:  model.BrushKindSolid,
			Solid: color,
		}
		opacity := convertScalar(value.Opacity.UnwrapOr(oneHundred))
		return model.Draw{
			Brush:   brush,
			Opacity: opacity,
		}, true
	case encoding.Stroke:
		var join curve.Join
		switch value.LineJoin.UnwrapOr(encoding.LineJoinBevel) {
		case encoding.LineJoinBevel:
			join = curve.BevelJoin
		case encoding.LineJoinRound:
			join = curve.RoundJoin
		case encoding.LineJoinMiter:
			join = curve.MiterJoin
		default:
			join = curve.BevelJoin
		}

		var cap curve.Cap
		switch value.LineCap.UnwrapOr(encoding.LineCapButt) {
		case encoding.LineCapButt:
			cap = curve.ButtCap
		case encoding.LineCapRound:
			cap = curve.RoundCap
		case encoding.LineCapSquare:
			cap = curve.SquareCap
		default:
			cap = curve.ButtCap
		}
		stroke := animation.KeyframedStroke{
			Width:      convertScalar(value.StrokeWidth),
			Join:       join,
			MiterLimit: maybe.Some(value.MiterLimit),
			Cap:        cap,
		}
		color := convertColor(value.Color)
		brush := model.Brush{
			Kind:  model.BrushKindSolid,
			Solid: color,
		}
		opacity := convertScalar(value.Opacity.UnwrapOr(oneHundred))
		return model.Draw{
			Stroke:  maybe.Some(stroke),
			Brush:   brush,
			Opacity: opacity,
		}, true

	case encoding.GradientFill:
		isRadial := value.GradientType == encoding.GradientTypeRadial
		startPoint := convertPoint(value.StartPoint)
		endPoint := convertPoint(value.EndPoint)
		gradient := animation.KeyframedGradient{
			IsRadial:   isRadial,
			StartPoint: startPoint,
			EndPoint:   endPoint,
			Stops:      convertGradientColors(value.Colors),
			ColorSpace: model.WorkingColorSpace,
		}
		brush := model.Brush{
			Kind:     model.BrushKindGradient,
			Gradient: gradient,
		}
		return model.Draw{
			Brush:   brush,
			Opacity: fixedValue(100.0),
		}, true
	case encoding.GradientStroke:
		var join curve.Join
		switch value.LineJoin.UnwrapOr(encoding.LineJoinRound) {
		case encoding.LineJoinBevel:
			join = curve.BevelJoin
		case encoding.LineJoinRound:
			join = curve.RoundJoin
		case encoding.LineJoinMiter:
			join = curve.MiterJoin
		default:
			join = curve.RoundJoin
		}

		var cap curve.Cap
		switch value.LineCap.UnwrapOr(encoding.LineCapRound) {
		case encoding.LineCapButt:
			cap = curve.ButtCap
		case encoding.LineCapRound:
			cap = curve.RoundCap
		case encoding.LineCapSquare:
			cap = curve.SquareCap
		default:
			cap = curve.RoundCap
		}
		stroke := animation.KeyframedStroke{
			Width:      convertScalar(value.StrokeWidth),
			Join:       join,
			MiterLimit: maybe.Some(value.MiterLimit),
			Cap:        cap,
		}
		isRadial := value.GradientType == encoding.GradientTypeRadial
		startPoint := convertPoint(value.StartPoint)
		endPoint := convertPoint(value.EndPoint)
		gradient := animation.KeyframedGradient{
			IsRadial:   isRadial,
			StartPoint: startPoint,
			EndPoint:   endPoint,
			Stops:      convertGradientColors(value.Colors),
			ColorSpace: model.WorkingColorSpace,
		}
		brush := model.Brush{
			Kind:     model.BrushKindGradient,
			Gradient: gradient,
		}
		return model.Draw{
			Stroke:  maybe.Some(stroke),
			Brush:   brush,
			Opacity: fixedValue(100.0),
		}, true
	default:
		return model.Draw{}, false
	}
}

func convertGradientColors(value encoding.GradientProperty) animation.KeyframedColorStops {
	count := value.NumColorStops
	if value.Value.Animated {
		n := len(value.Value.Keyframes)
		frames := make([]float64, n)
		easings := make([]animation.Curve, n)
		values := make([][]gfx.GradientStop, n)
		for i, value := range value.Value.Keyframes {
			frames[i], easings[i] = convertKeyframe(value.BaseKeyframe)
			values[i] = convertStops(value.Value, count)
		}
		return animation.KeyframedColorStops{
			Keyframes: animation.Keyframes[[]gfx.GradientStop]{
				Frames: frames,
				Curves: easings,
				Values: values,
			},
			ColorSpace: model.WorkingColorSpace,
		}
	} else {
		raw := convertStops(value.Value.Value, count)
		return animation.KeyframedColorStops{
			Keyframes: animation.Keyframes[[]gfx.GradientStop]{
				Frames: []float64{0},
				Curves: []animation.Curve{animation.CurveIdentity},
				Values: [][]gfx.GradientStop{raw},
			},
			ColorSpace: model.WorkingColorSpace,
		}
	}
}

func convertStops(value []float64, count int) []gfx.GradientStop {
	var stops []gfx.GradientStop
	var alphaStops [][2]float64
	for i := 0; i < (len(value)/4)*4; i += 4 {
		chunk := value[i : i+4]
		stops = append(stops, gfx.GradientStop{
			Offset: float32(chunk[0]),
			Color:  color.Make(model.ParsedColorSpace, chunk[1], chunk[2], chunk[3], 1.0),
		})
		if len(stops) >= count {
			// there is alpha data at the end of the list, which is a sequence
			// of (offset, alpha) pairs
			alphas := value[count*4:]
			for j := 0; j < (len(alphas)/2)*2; j += 2 {
				alphaStops = append(alphaStops, [2]float64(alphas[j:j+2]))
			}

			for j := range stops {
				stop := &stops[j]
				var last maybe.Option[[2]float64]
				for _, alphaStop := range alphaStops {
					b, alphaB := alphaStop[0], alphaStop[1]
					if a_, ok := last.Get(); ok {
						a, alphaA := a_[0], a_[1]
						x := float64(stop.Offset)
						t := normalizeToRange(a, b, x)
						var alphaInterp float64
						// todo: this is a hack to get alpha rendering with a
						// falloff similar to lottiefiles'
						switch {
						case x >= a && x <= b && t <= 0.25 && x <= 0.1:
							alphaInterp = alphaA
						case x >= a && x <= b && t >= 0.75 && x >= 0.9:
							alphaInterp = alphaB
						default:
							alphaInterp = mathutil.Lerp(alphaA, alphaB, t)
						}
						stop.Color.Values[3] = min(stop.Color.Values[3], alphaInterp)
					}
					last = maybe.Some([2]float64{b, alphaB})
				}
			}
			break
		}
	}
	if len(stops) > 1 {
		if stops[len(stops)-1].Offset < 1 {
			stops = append(stops, stops[len(stops)-1])
			stops[len(stops)-1].Offset = 1
		}
		if stops[0].Offset > 0 {
			stops = slices.Insert(stops, 0, stops[0])
			stops[0].Offset = 0
		}
	}
	return stops
}

func normalizeToRange(a, b, x float64) float64 {
	if a == b {
		// avoid division by zero
		return 0
	}
	return (x - a) / (b - a)
}

func convertColor(color encoding.ColorProperty) [3]animation.Keyframes[float64] {
	// Color values are always RGB, never RGBA. To control opacity, a separate
	// property has to be used.
	if color.Animated {
		return [3]animation.Keyframes[float64](
			convertMultiKeyframes(color.Keyframes, 3),
		)
	} else {
		return [3]animation.Keyframes[float64]{
			fixedValue(color.Value[0]),
			fixedValue(color.Value[1]),
			fixedValue(color.Value[2]),
		}
	}
}

func setupShapeLayer(source encoding.ShapeLayer, target *model.Layer) (int, maybe.Option[gfx.BlendMode]) {
	return setupLayerBase(source.VisualLayer, target)
}

func fixedValue[T any](v T) animation.Keyframes[T] {
	return animation.Keyframes[T]{
		Frames: []float64{0},
		Curves: []animation.Curve{animation.CurveIdentity},
		Values: []T{v},
	}
}

func convertKeyframe(k encoding.BaseKeyframe) (frame float64, easing animation.Curve) {
	if k.Hold {
		return k.Time, animation.CurveStatic(0)
	} else {
		return k.Time,
			animation.CurveCubicBezier{
				P1: maybe.Map(k.OutTangent, convertKeyframeHandle).UnwrapOr(curve.Pt(0, 0)),
				P2: maybe.Map(k.InTangent, convertKeyframeHandle).UnwrapOr(curve.Pt(1, 1)),
			}
	}
}

func toRadians(deg float64) float64 {
	return deg * (math.Pi / 180)
}
