// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"
	"testing"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

const defaultSize = 100

var (
	stopsGreenBlue = []gfx.GradientStop{
		{Offset: 0, Color: color.Make(color.SRGB, 0, 0.5, 0, 1)},
		{Offset: 1, Color: color.Make(color.SRGB, 0, 0, 1, 1)},
	}
	stopsGreenBlueWithAlpha = []gfx.GradientStop{
		{Offset: 0, Color: color.Make(color.SRGB, 0, 0.5, 0, 0.25)},
		{Offset: 1, Color: color.Make(color.SRGB, 0, 0, 1, 0.75)},
	}
	stopsBlueGreenRedYellow = []gfx.GradientStop{
		{
			Offset: 0.0,
			Color:  color.Make(color.SRGB, 0, 0, 1, 1),
		},
		{
			Offset: 0.33,
			Color:  color.Make(color.SRGB, 0, 0.5, 0, 1),
		},
		{
			Offset: 0.66,
			Color:  color.Make(color.SRGB, 1, 0, 0, 1),
		},
		{
			Offset: 1.0,
			Color:  color.Make(color.SRGB, 1, 1, 0, 1),
		},
	}
)

func crossedLineStar() curve.BezPath {
	var path curve.BezPath
	path.MoveTo(curve.Pt(50.0, 10.0))
	path.LineTo(curve.Pt(75.0, 90.0))
	path.LineTo(curve.Pt(10.0, 40.0))
	path.LineTo(curve.Pt(90.0, 40.0))
	path.LineTo(curve.Pt(25.0, 90.0))
	path.LineTo(curve.Pt(50.0, 10.0))
	return path
}

func TestLinearGradientOnThreeWideTiles(t *testing.T) {
	renderAndCompare(t, 600, 32, false, "gradient_on_3_wide_tiles", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(4, 4), curve.Pt(596, 28))
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(0, 0),
			End:        curve.Pt(600, 0),
			Stops:      stopsGreenBlue,
			Extend:     gfx.GradientExtendPad,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientTwoStops(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_linear_2_stops", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(10, 0),
			End:        curve.Pt(90, 0),
			Stops:      stopsGreenBlue,
			Extend:     gfx.GradientExtendPad,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientTwoStopsWithAlpha(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_linear_2_stops_with_alpha", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(10, 0),
			End:        curve.Pt(90, 0),
			Stops:      stopsGreenBlueWithAlpha,
			Extend:     gfx.GradientExtendPad,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientsDirections(t *testing.T) {
	for _, tt := range []struct {
		name  string
		start curve.Point
		end   curve.Point
	}{
		{"gradient_linear_negative_direction", curve.Pt(90.0, 0.0), curve.Pt(10.0, 0.0)},
		{"gradient_linear_with_downward_y", curve.Pt(20.0, 20.0), curve.Pt(80.0, 80.0)},
		{"gradient_linear_with_upward_y", curve.Pt(20.0, 80.0), curve.Pt(80.0, 20.0)},
		{"gradient_linear_vertical", curve.Pt(0.0, 10.0), curve.Pt(0.0, 90.0)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.LinearGradient{
					Start:      tt.start,
					End:        tt.end,
					Stops:      stopsGreenBlue,
					Extend:     gfx.GradientExtendPad,
					ColorSpace: color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestLinearGradientsExtends(t *testing.T) {
	for _, tt := range []struct {
		name   string
		extend gfx.GradientExtend
	}{
		{"gradient_linear_with_pad", gfx.GradientExtendPad},
		{"gradient_linear_with_repeat", gfx.GradientExtendRepeat},
		{"gradient_linear_with_reflect", gfx.GradientExtendReflect},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.LinearGradient{
					Start:      curve.Pt(40, 40),
					End:        curve.Pt(65, 95),
					Stops:      stopsBlueGreenRedYellow,
					Extend:     tt.extend,
					ColorSpace: color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestLinearGradient4Stops(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_linear_4_stops", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(10.0, 0.0),
			End:        curve.Pt(90.0, 0.0),
			Stops:      stopsBlueGreenRedYellow,
			Extend:     gfx.GradientExtendPad,
			ColorSpace: color.SRGB,
		}

		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientComplexShape(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_linear_complex_shape", func(ctx *Renderer) {
		path := crossedLineStar()
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(0.0, 0.0),
			End:        curve.Pt(100.0, 0.0),
			Stops:      stopsBlueGreenRedYellow,
			Extend:     gfx.GradientExtendPad,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(path, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientWithYRepeat(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_linear_with_y_repeat", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(47.5, 47.5),
			End:        curve.Pt(55.5, 54.5),
			Stops:      stopsBlueGreenRedYellow,
			Extend:     gfx.GradientExtendRepeat,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientWithYReflect(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_linear_with_y_reflect", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.LinearGradient{
			Start:      curve.Pt(47.5, 47.5),
			End:        curve.Pt(50.5, 52.5),
			Stops:      stopsBlueGreenRedYellow,
			Extend:     gfx.GradientExtendReflect,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestLinearGradientsWithTransform(t *testing.T) {
	for _, tt := range []struct {
		name      string
		transform curve.Affine
		start     curve.Point
		end       curve.Point
	}{
		{
			"gradient_linear_with_transform_identity",
			curve.Identity,
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},

		{
			"gradient_linear_with_transform_translate",
			curve.Translate(curve.Vec(25.0, 25.0)),
			curve.Pt(0.0, 0.0),
			curve.Pt(50.0, 50.0),
		},

		{
			"gradient_linear_with_transform_scale",
			curve.Scale(2.0, 2.0),
			curve.Pt(12.5, 12.5),
			curve.Pt(37.5, 37.5),
		},

		{
			"gradient_linear_with_transform_negative_scale",
			curve.Translate(curve.Vec(100.0, 100.0)).Mul(curve.Scale(-2.0, -2.0)),
			curve.Pt(12.5, 12.5),
			curve.Pt(37.5, 37.5),
		},

		{
			"gradient_linear_with_transform_scale_and_translate",
			curve.NewAffine([6]float64{
				2.0, 0.0, 0.0,
				2.0, 25.0, 25.0,
			}),
			curve.Pt(0.0, 0.0),
			curve.Pt(25.0, 25.0),
		},

		{
			"gradient_linear_with_transform_rotate_1",
			curve.RotateAbout(math.Pi/4.0, curve.Pt(50.0, 50.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},

		{
			"gradient_linear_with_transform_rotate_2",
			curve.RotateAbout(-math.Pi/4.0, curve.Pt(50.0, 50.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},

		{
			"gradient_linear_with_transform_scaling_non_uniform",
			curve.Scale(1.0, 2.0),
			curve.Pt(25.0, 12.5),
			curve.Pt(75.0, 37.5),
		},

		{
			"gradient_linear_with_transform_skew_x_1",
			curve.Translate(curve.Vec(-50.0, 0.0)).Mul(curve.Skew(1, 0.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},

		{
			"gradient_linear_with_transform_skew_x_2",
			curve.Translate(curve.Vec(50.0, 0.0)).Mul(curve.Skew(-1, 0.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},

		{
			"gradient_linear_with_transform_skew_y_1",
			curve.Translate(curve.Vec(0.0, 50.0)).Mul(curve.Skew(0.0, -1)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},

		{
			"gradient_linear_with_transform_skew_y_2",
			curve.Translate(curve.Vec(0.0, -50.0)).Mul(curve.Skew(0.0, 1)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(tt.start, tt.end)
				gradient := &gfx.LinearGradient{
					Start:      tt.start,
					End:        tt.end,
					Stops:      stopsBlueGreenRedYellow,
					Extend:     gfx.GradientExtendPad,
					ColorSpace: color.SRGB,
				}
				ctx.Fill(rect, tt.transform, gfx.NonZero, gradient)
			})
		})
	}
}

func TestRadialGradientsSimple(t *testing.T) {
	for _, tt := range []struct {
		name  string
		stops []gfx.GradientStop
	}{
		{"gradient_radial_2_stops", stopsGreenBlue},
		{"gradient_radial_4_stops", stopsBlueGreenRedYellow},
		{"gradient_radial_2_stops_with_alpha", stopsGreenBlueWithAlpha},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.RadialGradient{
					StartCenter: curve.Pt(50, 50),
					StartRadius: 10,
					EndCenter:   curve.Pt(50, 50),
					EndRadius:   40,
					Stops:       tt.stops,
					Extend:      gfx.GradientExtendPad,
					ColorSpace:  color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestRadialGradientsWithOffsets(t *testing.T) {
	for _, tt := range []struct {
		name  string
		point curve.Point
	}{
		{"gradient_radial_center_offset_top_left", curve.Pt(30.0, 30.0)},
		{"gradient_radial_center_offset_top_right", curve.Pt(70.0, 30.0)},
		{"gradient_radial_center_offset_bottom_left", curve.Pt(30.0, 70.0)},
		{"gradient_radial_center_offset_bottom_right", curve.Pt(70.0, 70.0)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.RadialGradient{
					StartCenter: tt.point,
					StartRadius: 2,
					EndCenter:   curve.Pt(50, 50),
					EndRadius:   40,
					Stops:       stopsBlueGreenRedYellow,
					Extend:      gfx.GradientExtendRepeat,
					ColorSpace:  color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestRadialGradientsWithExtends(t *testing.T) {
	for _, tt := range []struct {
		name   string
		extend gfx.GradientExtend
	}{
		// FIXME rename from spread_method to extend
		{"gradient_radial_spread_method_pad", gfx.GradientExtendPad},
		{"gradient_radial_spread_method_reflect", gfx.GradientExtendReflect},
		{"gradient_radial_spread_method_repeat", gfx.GradientExtendRepeat},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.RadialGradient{
					StartCenter: curve.Pt(50, 50),
					StartRadius: 20,
					EndCenter:   curve.Pt(50, 50),
					EndRadius:   25,
					Stops:       stopsBlueGreenRedYellow,
					Extend:      tt.extend,
					ColorSpace:  color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestRadialGradientC0Bigger(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_radial_circle_1_bigger_radius", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.RadialGradient{
			StartCenter: curve.Pt(50, 50),
			StartRadius: 40,
			EndCenter:   curve.Pt(50, 50),
			EndRadius:   10,
			Stops:       stopsBlueGreenRedYellow,
			Extend:      gfx.GradientExtendPad,
			ColorSpace:  color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestRadialGradientsNonOverlapping(t *testing.T) {
	for _, tt := range []struct {
		name   string
		radius float32
	}{
		{"gradient_radial_non_overlapping_same_size", 20},
		{"gradient_radial_non_overlapping_c0_smaller", 15},
		{"gradient_radial_non_overlapping_c0_larger", 25},
		{"gradient_radial_non_overlapping_cone", 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.RadialGradient{
					StartCenter: curve.Pt(30, 50),
					StartRadius: tt.radius,
					EndCenter:   curve.Pt(70, 50),
					EndRadius:   20,
					Stops:       stopsBlueGreenRedYellow,
					Extend:      gfx.GradientExtendPad,
					ColorSpace:  color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestRadialGradientComplexShape(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_radial_complex_shape", func(ctx *Renderer) {
		path := crossedLineStar()
		gradient := &gfx.RadialGradient{
			StartCenter: curve.Pt(50, 50),
			StartRadius: 5,
			EndCenter:   curve.Pt(50, 50),
			EndRadius:   35,
			Stops:       stopsBlueGreenRedYellow,
			Extend:      gfx.GradientExtendPad,
			ColorSpace:  color.SRGB,
		}
		ctx.Fill(path, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestRadialGradientsWithTransforms(t *testing.T) {
	for _, tt := range []struct {
		name      string
		transform curve.Affine
		p0        curve.Point
		p1        curve.Point
	}{
		{
			"gradient_radial_with_transform_identity",
			curve.Identity,
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
		{
			"gradient_radial_with_transform_translate",
			curve.Translate(curve.Vec(25.0, 25.0)),
			curve.Pt(0.0, 0.0),
			curve.Pt(50.0, 50.0),
		},
		{
			"gradient_radial_with_transform_scale",
			curve.Scale(2.0, 2.0),
			curve.Pt(12.5, 12.5),
			curve.Pt(37.5, 37.5),
		},
		{
			"gradient_radial_with_transform_negative_scale",
			curve.Translate(curve.Vec(100.0, 100.0)).Mul(curve.Scale(-2.0, -2.0)),
			curve.Pt(12.5, 12.5),
			curve.Pt(37.5, 37.5),
		},
		{
			"gradient_radial_with_transform_scale_and_translate",
			curve.NewAffine([6]float64{2.0, 0.0, 0.0, 2.0, 25.0, 25.0}),
			curve.Pt(0.0, 0.0),
			curve.Pt(25.0, 25.0),
		},
		{
			"gradient_radial_with_transform_rotate_1",
			curve.RotateAbout(math.Pi/4.0, curve.Pt(50.0, 50.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
		{
			"gradient_radial_with_transform_rotate_2",
			curve.RotateAbout(-math.Pi/4.0, curve.Pt(50.0, 50.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
		{
			"gradient_radial_with_transform_scale_non_uniform",
			curve.Scale(1.0, 2.0),
			curve.Pt(25.0, 12.5),
			curve.Pt(75.0, 37.5),
		},
		{
			"gradient_radial_with_transform_skew_x_1",
			curve.Translate(curve.Vec(-50.0, 0.0)).Mul(curve.Skew(1, 0.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
		{
			"gradient_radial_with_transform_skew_x_2",
			curve.Translate(curve.Vec(50.0, 0.0)).Mul(curve.Skew(-1, 0.0)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
		{
			"gradient_radial_with_transform_skew_y_1",
			curve.Translate(curve.Vec(0.0, 50.0)).Mul(curve.Skew(0.0, -1)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
		{
			"gradient_radial_with_transform_skew_y_2",
			curve.Translate(curve.Vec(0.0, -50.0)).Mul(curve.Skew(0.0, 1)),
			curve.Pt(25.0, 25.0),
			curve.Pt(75.0, 75.0),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(tt.p0, tt.p1)
				point := curve.Pt((tt.p0.X+tt.p1.X)/2, (tt.p0.Y+tt.p1.Y)/2)
				gradient := &gfx.RadialGradient{
					StartCenter: point,
					StartRadius: 5,
					EndCenter:   point,
					EndRadius:   35,
					Stops:       stopsBlueGreenRedYellow,
					Extend:      gfx.GradientExtendPad,
					ColorSpace:  color.SRGB,
				}
				ctx.Fill(rect, tt.transform, gfx.NonZero, gradient)
			})
		})
	}
}

func TestSweepGradientsBasic(t *testing.T) {
	for _, tt := range []struct {
		name   string
		stops  []gfx.GradientStop
		center curve.Point
	}{
		{"gradient_sweep_2_stops", stopsGreenBlue, curve.Pt(50, 50)},
		{"gradient_sweep_2_stops_with_alpha", stopsGreenBlueWithAlpha, curve.Pt(50, 50)},
		{"gradient_sweep_4_stops", stopsBlueGreenRedYellow, curve.Pt(50, 50)},
		{"gradient_sweep_not_in_center", stopsGreenBlue, curve.Pt(30, 30)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.SweepGradient{
					Center:     tt.center,
					StartAngle: 0,
					EndAngle:   2 * math.Pi,
					Stops:      tt.stops,
					Extend:     gfx.GradientExtendPad,
					ColorSpace: color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestSweepGradientsWithExtends(t *testing.T) {
	for _, tt := range []struct {
		name   string
		extend gfx.GradientExtend
	}{
		{"gradient_sweep_extend_pad", gfx.GradientExtendPad},
		{"gradient_sweep_extend_repeat", gfx.GradientExtendRepeat},
		{"gradient_sweep_extend_reflect", gfx.GradientExtendReflect},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
				gradient := &gfx.SweepGradient{
					Center:     curve.Pt(50, 50),
					StartAngle: 150 * math.Pi / 180,
					EndAngle:   210 * math.Pi / 180,
					Stops:      stopsBlueGreenRedYellow,
					Extend:     tt.extend,
					ColorSpace: color.SRGB,
				}
				ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
			})
		})
	}
}

func TestSweepGradientsWithTransforms(t *testing.T) {
	for _, tt := range []struct {
		name      string
		transform curve.Affine
		p0        curve.Point
		p1        curve.Point
	}{
		{
			"gradient_sweep_with_transform_identity",
			curve.Identity,
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
		{
			"gradient_sweep_with_transform_translate",
			curve.Translate(curve.Vec(25, 25)),
			curve.Pt(0, 0),
			curve.Pt(50, 50),
		},
		{
			"gradient_sweep_with_transform_scale",
			curve.Scale(2, 2),
			curve.Pt(12.5, 12.5),
			curve.Pt(37.5, 37.5),
		},
		{
			"gradient_sweep_with_transform_negative_scale",
			curve.Translate(curve.Vec(100, 100)).Mul(curve.Scale(-2, -2)),
			curve.Pt(12.5, 12.5),
			curve.Pt(37.5, 37.5),
		},
		{
			"gradient_sweep_with_transform_scale_and_translate",
			curve.NewAffine([6]float64{2, 0, 0, 2, 25, 25}),
			curve.Pt(0, 0),
			curve.Pt(25, 25),
		},
		{
			"gradient_sweep_with_transform_rotate_1",
			curve.RotateAbout(math.Pi/3, curve.Pt(50, 50)),
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
		{
			"gradient_sweep_with_transform_rotate_2",
			curve.RotateAbout(-math.Pi/3, curve.Pt(50, 50)),
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
		{
			"gradient_sweep_with_transform_scale_non_uniform",
			curve.Scale(1, 2),
			curve.Pt(25, 12.5),
			curve.Pt(75, 37.5),
		},
		{
			"gradient_sweep_with_transform_skew_x_1",
			curve.Translate(curve.Vec(-50, 0)).Mul(curve.Skew(1, 0)),
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
		{
			"gradient_sweep_with_transform_skew_x_2",
			curve.Translate(curve.Vec(50, 0)).Mul(curve.Skew(-1, 0)),
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
		{
			"gradient_sweep_with_transform_skew_y_1",
			curve.Translate(curve.Vec(0, 50)).Mul(curve.Skew(0, -1)),
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
		{
			"gradient_sweep_with_transform_skew_y_2",
			curve.Translate(curve.Vec(0, -50)).Mul(curve.Skew(0, 1)),
			curve.Pt(25, 25),
			curve.Pt(75, 75),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			renderAndCompare(t, defaultSize, defaultSize, false, tt.name, func(ctx *Renderer) {
				rect := curve.NewRectFromPoints(tt.p0, tt.p1)
				point := curve.Pt((tt.p0.X+tt.p1.X)/2, (tt.p0.Y+tt.p1.Y)/2)
				gradient := &gfx.SweepGradient{
					Center:     point,
					StartAngle: 150 * math.Pi / 180,
					EndAngle:   210 * math.Pi / 180,
					Stops:      stopsBlueGreenRedYellow,
					Extend:     gfx.GradientExtendPad,
					ColorSpace: color.SRGB,
				}
				ctx.Fill(rect, tt.transform, gfx.NonZero, gradient)
			})
		})
	}
}

func TestSweepGradientComplexShape(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_sweep_complex_shape", func(ctx *Renderer) {
		path := crossedLineStar()
		gradient := &gfx.SweepGradient{
			Center:     curve.Pt(50, 50),
			StartAngle: 0,
			EndAngle:   2 * math.Pi,
			Stops:      stopsBlueGreenRedYellow,
			Extend:     gfx.GradientExtendPad,
			ColorSpace: color.SRGB,
		}
		ctx.Fill(path, curve.Identity, gfx.NonZero, gradient)
	})
}

func TestRadialGradientSmallerR1WithReflect(t *testing.T) {
	renderAndCompare(t, defaultSize, defaultSize, false, "gradient_radial_smaller_r1_with_reflect", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(10, 10), curve.Pt(90, 90))
		gradient := &gfx.RadialGradient{
			StartCenter: curve.Pt(30, 50),
			StartRadius: 20,
			EndCenter:   curve.Pt(70, 50),
			EndRadius:   5,
			Stops:       stopsBlueGreenRedYellow,
			Extend:      gfx.GradientExtendReflect,
			ColorSpace:  color.SRGB,
		}
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gradient)
	})
}

func BenchmarkMakeGradientLUT(b *testing.B) {
	ranges := encodeStops(stopsBlueGreenRedYellow, color.Oklab)
	for b.Loop() {
		makeGradientLUT(ranges)
	}
}
