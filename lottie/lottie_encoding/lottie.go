// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_encoding

import (
	"fmt"
	"reflect"

	"honnef.co/go/gutter/maybe"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

func decodeTyped(d *jsontext.Decoder, a *any, o json.Options, m map[any]reflect.Type) error {
	v, err := d.ReadValue()
	if err != nil {
		return err
	}
	var typ struct {
		Type any `json:"ty"`
	}
	if err := json.Unmarshal(v, &typ); err != nil {
		return err
	}
	t, ok := m[typ.Type]
	if !ok {
		*a = nil
		return nil
	}
	reflect.ValueOf(a).Elem().Set(reflect.New(t).Elem())
	return json.Unmarshal(v, a, o)
}

var layerTypes = map[any]reflect.Type{
	// We have to use float64 keys because the key gets unmarshaled into any,
	// which uses float64, not int
	float64(0): reflect.TypeOf(PrecompositionLayer{}),
	float64(1): reflect.TypeOf(SolidLayer{}),
	float64(2): reflect.TypeOf(ImageLayer{}),
	float64(3): reflect.TypeOf(NullLayer{}),
	float64(4): reflect.TypeOf(ShapeLayer{}),
}

var graphicElementTypes = map[any]reflect.Type{
	"el": reflect.TypeOf(Ellipse{}),
	"fl": reflect.TypeOf(Fill{}),
	"gf": reflect.TypeOf(GradientFill{}),
	"gs": reflect.TypeOf(GradientStroke{}),
	"gr": reflect.TypeOf(Group{}),
	"sh": reflect.TypeOf(Path{}),
	"sr": reflect.TypeOf(Polystar{}),
	"rc": reflect.TypeOf(Rectangle{}),
	"st": reflect.TypeOf(Stroke{}),
	"tr": reflect.TypeOf(TransformShape{}),
	"tm": reflect.TypeOf(TrimPath{}),
}

var unmarshalLayer = json.UnmarshalFuncV2(func(d *jsontext.Decoder, a *AnyLayer, o json.Options) error {
	return decodeTyped(d, (*any)(a), o, layerTypes)
})

var unmarshalGraphicElement = json.UnmarshalFuncV2(func(d *jsontext.Decoder, a *AnyGraphicElement, o json.Options) error {
	return decodeTyped(d, (*any)(a), o, graphicElementTypes)
})

var unmarshalAsset = json.UnmarshalFuncV2(func(d *jsontext.Decoder, a *AnyAsset, o json.Options) error {

	var asset struct {
		Asset
		Layers []AnyLayer `json:"layers"`
		SlottableObject
		Width    float64    `json:"w"`
		Height   float64    `json:"h"`
		FileName string     `json:"p"`
		FilePath string     `json:"u"`
		Embedded IntBoolean `json:"e"`
	}
	v, err := d.ReadValue()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(v, &asset, o); err != nil {
		return err
	}
	if asset.Layers != nil {
		*a = Precomposition{
			Asset:  asset.Asset,
			Layers: asset.Layers,
		}
	} else if asset.Width != 0 {
		*a = Image{
			Asset:           asset.Asset,
			SlottableObject: asset.SlottableObject,
			Width:           asset.Width,
			Height:          asset.Height,
			FileName:        asset.FileName,
			FilePath:        asset.FilePath,
			Embedded:        asset.Embedded,
		}
	} else {
		*a = nil
	}
	return nil

})

func Parse(b []byte) (*Animation, error) {
	var anim Animation
	err := json.Unmarshal(
		b,
		&anim,
		json.WithUnmarshalers(
			json.NewUnmarshalers(unmarshalLayer, unmarshalGraphicElement, unmarshalAsset),
		),
	)
	return &anim, err
}

type BlendMode int

const (
	BlendModeNormal     BlendMode = 0
	BlendModeMultiply   BlendMode = 1
	BlendModeScreen     BlendMode = 2
	BlendModeOverlay    BlendMode = 3
	BlendModeDarken     BlendMode = 4
	BlendModeLighten    BlendMode = 5
	BlendModeColorDodge BlendMode = 6
	BlendModeColorBurn  BlendMode = 7
	BlendModeHardLight  BlendMode = 8
	BlendModeSoftLight  BlendMode = 9
	BlendModeDifference BlendMode = 10
	BlendModeExclusion  BlendMode = 11
	BlendModeHue        BlendMode = 12
	BlendModeSaturation BlendMode = 13
	BlendModeColor      BlendMode = 14
	BlendModeLuminosity BlendMode = 15
	BlendModeAdd        BlendMode = 16
	BlendModeHardMix    BlendMode = 17
)

type StrokeDashType string

const (
	StrokeDashTypeDash   StrokeDashType = "d"
	StrokeDashTypeGap    StrokeDashType = "g"
	StrokeDashTypeOffset StrokeDashType = "o"
)

type StarType int

const (
	StarTypeStar    StarType = 1
	StarTypePolygon StarType = 2
)

type MaskMode string

const (
	MaskModeNone       MaskMode = "n"
	MaskModeAdd        MaskMode = "a"
	MaskModeSubtract   MaskMode = "s"
	MaskModeIntersect  MaskMode = "i"
	MaskModeLighten    MaskMode = "l"
	MaskModeDarken     MaskMode = "d"
	MaskModeDifference MaskMode = "f"
)

type LineJoin int

const (
	LineJoinMiter LineJoin = 1
	LineJoinRound LineJoin = 2
	LineJoinBevel LineJoin = 3
)

type TrimMultipleShapes int

const (
	TrimMultipleShapesParallel   TrimMultipleShapes = 1
	TrimMultipleShapesSequential TrimMultipleShapes = 2
)

type GradientType int

const (
	GradientTypeLinear GradientType = 1
	GradientTypeRadial GradientType = 2
)

type ShapeDirection int

const (
	ShapeDirectionNormal   ShapeDirection = 1
	ShapeDirectionReversed ShapeDirection = 3
)

type FillRule int

const (
	FillRuleNonZero FillRule = 1
	FillRuleEvenOdd FillRule = 2
)

type MatteMode int

const (
	MatteModeNormal        MatteMode = 0
	MatteModeAlpha         MatteMode = 1
	MatteModeInvertedAlpha MatteMode = 2
	MatteModeLuma          MatteMode = 3
	MatteModeInvertedLuma  MatteMode = 4
)

type LineCap int

const (
	LineCapButt   LineCap = 1
	LineCapRound  LineCap = 2
	LineCapSquare LineCap = 3
)

type AnimatableProperty[T, K any] struct {
	Animated  IntBoolean
	Value     T
	Keyframes []K
	// Number of components in the value arrays. We're supposed to truncate or
	// pad to this number. Why don't arrays contain the correct number of
	// elements in the first place? Who knows... But we've seen real files where
	// scaling in a transformation had 3 array values instead of the required 2.
	Length maybe.Option[int]
}

func (prop *AnimatableProperty[T, K]) UnmarshalJSON(data []byte) error {
	var animated struct {
		Animated IntBoolean `json:"a"`
	}
	if err := json.Unmarshal(data, &animated); err != nil {
		return err
	}
	if animated.Animated {
		var value struct {
			Value  []K               `json:"k"`
			Length maybe.Option[int] `json:"l"`
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		prop.Animated = true
		prop.Keyframes = value.Value
	} else {
		var value struct {
			Value  T                 `json:"k"`
			Length maybe.Option[int] `json:"l"`
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		prop.Animated = false
		prop.Value = value.Value
	}
	return nil
}

type Animation struct {
	VisualObject
	Layers      []AnyLayer `json:"layers"`
	SpecVersion int        `json:"ver"`
	Framerate   float64    `json:"fr"`
	// Frame the animation starts at
	InPoint float64 `json:"ip"`
	// Frame the animation stops or loops at
	OutPoint float64    `json:"op"`
	Width    int        `json:"w"`
	Height   int        `json:"h"`
	Assets   []AnyAsset `json:"assets"`
	Markers  []Marker
	Slots    Slot `json:"slots"`
}

type NullLayer struct {
	VisualLayer
	Type int `json:"ty"` // 3
}

type UnknownLayer struct {
	Type int `json:"ty"` // not 0, 1, 2, 3, 4
}

type AnyGraphicElement any

type ShapeLayer struct {
	VisualLayer
	Type   int                 `json:"ty"` // 4
	Shapes []AnyGraphicElement `json:"shapes"`
}

type Layer struct {
	VisualObject
	Hidden      bool              `json:"hd"`
	Type        int               `json:"ty"`
	Index       int               `json:"ind"`
	ParentIndex maybe.Option[int] `json:"parent"`
	InPoint     float64           `json:"ip"`
	OutPoint    float64           `json:"op"`
}

type PrecompositionLayer struct {
	VisualLayer
	Type        int                          `json:"ty"` // 0
	ReferenceID string                       `json:"refId"`
	Width       float64                      `json:"w"`
	Height      float64                      `json:"h"`
	TimeRemap   maybe.Option[ScalarProperty] `json:"tm"`
}

type ImageLayer struct {
	VisualLayer
	Type        int    `json:"ty"` // 2
	ReferenceID string `json:"refId"`
}

type AnyLayer any

type SolidLayer struct {
	VisualLayer
	Type   int      `json:"ty"` // 1
	Width  int      `json:"sw"`
	Height int      `json:"sh"`
	Color  HexColor `json:"sc"`
}

type VisualLayer struct {
	Layer
	Transform                     Transform               `json:"ks"`
	AutoOrient                    IntBoolean              `json:"ao"` // defaults to 0
	MatteMode                     maybe.Option[MatteMode] `json:"tt"`
	MatteParent                   maybe.Option[int]       `json:"tp"`
	MasksProperties               []Mask                  `json:"marksProperties"`
	TransformBeforeMask           IntBoolean              `json:"ct"`
	TransformBeforeMaskDeprecated string                  `json:"cp"`
	BlendMode                     BlendMode               `json:"bm"`
	MotionBlur                    bool                    `json:"mb"`
	HasMask                       maybe.Option[bool]      `json:"hasMask"`
	MatteTarget                   IntBoolean              `json:"td"`
	StartTime                     float64                 `json:"st"` // defaults to 0
	TimeStretch                   maybe.Option[float64]   `json:"sr"` // defaults to 1
}

type Numbers []float64

var _ json.UnmarshalerV2 = (*Numbers)(nil)

func (arr *Numbers) UnmarshalJSONV2(dec *jsontext.Decoder, o json.Options) error {
	v, err := dec.ReadValue()
	if err != nil {
		return err
	}
	switch v.Kind() {
	case '[':
		return json.Unmarshal(v, (*[]float64)(arr))
	case '0':
		*arr = []float64{0}
		return json.Unmarshal(v, &(*arr)[0])
	default:
		return fmt.Errorf("expected array or number, got %c", v.Kind())
	}
}

type EasingHandle struct {
	X Numbers `json:"x"`
	Y Numbers `json:"y"`
}

type SimpleKeyframe[T any] struct {
	BaseKeyframe
	Value T `json:"s"`
}

type BaseKeyframe struct {
	Time       float64                    `json:"t"` // defaults to 0
	Hold       IntBoolean                 `json:"h"` // defaults to 0
	InTangent  maybe.Option[EasingHandle] `json:"i"`
	OutTangent maybe.Option[EasingHandle] `json:"o"`
}

type ScalarProperty struct {
	SlottableProperty
	AnimatableProperty[float64, SimpleKeyframe[[]float64]]
}

// XXX support position keyframe easing
// type PositionKeyframe struct {
// 	SimpleKeyframe[[]float64]
// 	InTangent  []float64 `json:"ti"`
// 	OutTangent []float64 `json:"to"`
// }

type ColorProperty struct {
	SlottableProperty
	AnimatableProperty[Color, SimpleKeyframe[Color]]
}

type PositionProperty = VectorProperty

type Color [3]float64

func (v *Color) UnmarshalJSON(b []byte) error {
	var values []float64
	if err := json.Unmarshal(b, &values); err != nil {
		return err
	}
	switch len(values) {
	case 0:
		*v = Color{}
	case 1:
		*v = Color{values[0], 0, 0}
	case 2:
		*v = Color{values[0], values[1], 0}
	default:
		*v = Color{values[0], values[1], values[2]}
	}
	return nil
}

type Vec2 [2]float64

func (v *Vec2) UnmarshalJSON(b []byte) error {
	var values []float64
	if err := json.Unmarshal(b, &values); err != nil {
		return err
	}
	switch len(values) {
	case 0:
		*v = Vec2{}
	case 1:
		*v = Vec2{values[0], 0}
	default:
		*v = Vec2{values[0], values[1]}
	}
	return nil
}

type VectorProperty struct {
	SlottableProperty
	AnimatableProperty[Vec2, SimpleKeyframe[Vec2]]
}

type SplitPosition struct {
	Split bool           `json:"s"` // true
	X     ScalarProperty `json:"x"`
	Y     ScalarProperty `json:"y"`
}

type BezierProperty struct {
	AnimatableProperty[Bezier, SimpleKeyframe[[]Bezier]]
}

type GradientProperty struct {
	NumColorStops int                                                    `json:"p"`
	Value         AnimatableProperty[Gradient, SimpleKeyframe[Gradient]] `json:"k"`
}

type SplittablePositionProperty struct {
	Split bool `json:"s"`
	PositionProperty
	SplitPosition
}

func (v *SplittablePositionProperty) UnmarshalJSON(data []byte) error {
	var split struct {
		Split bool `json:"s"`
	}
	if err := json.Unmarshal(data, &split); err != nil {
		return err
	}
	if split.Split {
		v.Split = true
		return json.Unmarshal(data, &v.SplitPosition)
	} else {
		v.Split = false
		return json.Unmarshal(data, &v.PositionProperty)
	}
}

type IntBoolean bool

func (v *IntBoolean) UnmarshalJSON(data []byte) error {
	var n int
	err := json.Unmarshal(data, &n)
	*v = n != 0
	return err
}

type HexColor string
type DataURL string
type Gradient []float64

type Bezier struct {
	Closed      bool   `json:"c"` // defaults to false
	InTangents  []Vec2 `json:"i"` // defaults to []
	OutTangents []Vec2 `json:"o"` // defaults to []
	Vertices    []Vec2 `json:"v"` // defaults to []
}

type Slot struct {
	PropertyValue any `json:"p"`
}

type Transform struct {
	// Where we don't use maybe.Option, the zero value is the default value we'd
	// use.

	AnchorPoint PositionProperty             `json:"a"`
	Position    SplittablePositionProperty   `json:"p"`
	Rotation    ScalarProperty               `json:"r"`
	Scale       maybe.Option[VectorProperty] `json:"s"`
	Opacity     maybe.Option[ScalarProperty] `json:"o"`
	Skew        ScalarProperty               `json:"sk"`
	SkewAxis    ScalarProperty               `json:"sa"`
}

type Mask struct {
	Mode    MaskMode                     `json:"mode"` // defaults to "i"
	Opacity maybe.Option[ScalarProperty] `json:"o"`    // defaults to 100
	Shape   maybe.Option[BezierProperty] `json:"pt"`
}

type SlottableObject struct {
	SlotID string `json:"sid"`
}

type Marker struct {
	Comment  string  `json:"cm"`
	Time     float64 `json:"tm"`
	Duration float64 `json:"dr"`
}

type SlottableProperty struct {
	SlottableObject
}

type VisualObject struct {
	Name      string `json:"nm"`
	MatchName string `json:"mn"`
}

type Fill struct {
	ShapeStyle
	Type     string        `json:"ty"` // "fl"
	Color    ColorProperty `json:"c"`
	FillRule FillRule      `json:"r"`
}

type Ellipse struct {
	BaseShape
	Type     string           `json:"ty"` // "el"
	Position PositionProperty `json:"p"`
	Size     VectorProperty   `json:"s"`
}

type ShapeStyle struct {
	GraphicElement
	Opacity maybe.Option[ScalarProperty] `json:"o"`
}

type Path struct {
	BaseShape
	Type  string         `json:"ty"` // "sh"
	Shape BezierProperty `json:"ks"`
}

type GradientStroke struct {
	ShapeStyle
	BaseStroke
	BaseGradient
	Type string `json:"ty"` // "gs"
}

type BaseGradient struct {
	Colors          GradientProperty             `json:"g"`
	StartPoint      PositionProperty             `json:"s"`
	EndPoint        PositionProperty             `json:"e"`
	GradientType    GradientType                 `json:"t"`
	HighlightLength maybe.Option[ScalarProperty] `json:"h"`
	HighlightAngle  maybe.Option[ScalarProperty] `json:"a"`
}

type BaseStroke struct {
	LineCap              maybe.Option[LineCap]        `json:"lc"` // defaults to 2
	LineJoin             maybe.Option[LineJoin]       `json:"lj"` // defaults to 2
	MiterLimit           float64                      `json:"ml"` // defaults to 0
	AnimatableMiterLimit maybe.Option[ScalarProperty] `json:"ml2"`
	StrokeWidth          ScalarProperty               `json:"w"`
	Dashes               []StrokeDash                 `json:"d"`
}

type GradientFill struct {
	ShapeStyle
	BaseGradient
	Type     string   `json:"gf"`
	FillRule FillRule `json:"r"`
}

type StrokeDash struct {
	VisualObject
	DashType StrokeDashType               `json:"n"` // defaults to "d"
	Length   maybe.Option[ScalarProperty] `json:"v"`
}

type BaseShape struct {
	GraphicElement
	Direction ShapeDirection
}

type ShapeModifier struct {
	GraphicElement
}

type Stroke struct {
	ShapeStyle
	BaseStroke
	Type  string        `json:"ty"` // "st"
	Color ColorProperty `json:"c"`
}

type GraphicElement struct {
	VisualObject
	Hidden bool   `json:"hd"`
	Type   string `json:"ty"`
}

func (g GraphicElement) IsHidden() bool { return g.Hidden }

type Polystar struct {
	BaseShape
	Type           string                       `json:"ty"` // "sr"
	Position       PositionProperty             `json:"p"`
	OuterRadius    ScalarProperty               `json:"or"`
	OuterRoundness ScalarProperty               `json:"os"`
	Rotation       ScalarProperty               `json:"r"`
	Points         ScalarProperty               `json:"pt"`
	StarType       StarType                     `json:"sy"` // defaults to 1
	InnerRadius    maybe.Option[ScalarProperty] `json:"ir"`
	InnerRoundness maybe.Option[ScalarProperty] `json:"is"`
	// If StarType == 1, InnerRadius and InnerRoundness are required
}

type TransformShape struct {
	GraphicElement
	Transform
	Type string `json:"ty"` // "tr"
}

type Group struct {
	GraphicElement
	Type          string              `json:"ty"` // "gr"
	NumProperties float64             `json:"np"`
	Shapes        []AnyGraphicElement `json:"it"`
}

type Rectangle struct {
	BaseShape
	Type     string           `json:"ty"` // "rc"
	Position PositionProperty `json:"p"`
	Size     VectorProperty   `json:"s"`
	Rounded  ScalarProperty   `json:"r"`
}

type TrimPath struct {
	ShapeModifier
	Type     string             `json:"ty"` // "tm"
	Start    ScalarProperty     `json:"s"`
	End      ScalarProperty     `json:"e"`
	Offset   ScalarProperty     `json:"o"`
	Multiple TrimMultipleShapes `json:"m"`
}

type UnknownShape struct {
	Type string `json:"ty"`
}

type AnyAsset any

type Precomposition struct {
	Asset
	Layers []AnyLayer `json:"layers"`
}

type Asset struct {
	VisualObject
	ID string `json:"id"`
}

type Image struct {
	Asset
	SlottableObject
	Width    float64    `json:"w"`
	Height   float64    `json:"h"`
	FileName string     `json:"p"`
	FilePath string     `json:"u"`
	Embedded IntBoolean `json:"e"`
}
