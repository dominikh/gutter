// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/safeish"
)

var writeGolden = flag.Bool("write-golden", false, "Write golden files")

func getCtx(width, height uint16, transparent bool) *Renderer {
	ctx := NewRenderer(width, height)
	if !transparent {
		ctx.Fill(
			curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(float64(width), float64(height))),
			curve.Identity,
			gfx.NonZero,
			gfx.Solid(color.Make(color.LinearSRGB, 1, 1, 1, 1)),
		)
	}
	return ctx
}

func renderAndCompare(t *testing.T, width, height uint16, transparent bool, name string, fn func(ctx *Renderer)) {
	t.Helper()

	ctx := getCtx(width, height, transparent)
	fn(ctx)
	compareRendered(t, ctx, name)
}

func render(ctx *Renderer) []gfx.PlainColor {
	pixmap := make([]gfx.PlainColor, int(ctx.width)*int(ctx.height))
	packer := &PackerFloat32{
		Out:    pixmap,
		Width:  int(ctx.width),
		Height: int(ctx.height),
	}
	ctx.Render(packer)
	return pixmap
}

func compareRendered(t *testing.T, ctx *Renderer, name string) {
	t.Helper()

	pixmap := render(ctx)
	img := image.NewNRGBA64(image.Rect(0, 0, int(ctx.width), int(ctx.height)))
	writeF32AsU16(pixmap, safeish.Cast[[][8]uint8](img.Pix))

	if *writeGolden {
		f, err := os.Create("./testdata/golden/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		return
	}

	f, err := os.Open("./testdata/golden/" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	var failed bool
	var worstDiff int64
	var numWrong int
	if golden.Bounds().Dx() != int(ctx.width) || golden.Bounds().Dy() != int(ctx.height) {
		t.Errorf("got size (%d, %d), want (%d, %d)", ctx.width, ctx.height, golden.Bounds().Dx(), golden.Bounds().Dy())
		failed = true
	} else {
		for x := range ctx.width {
			for y := range ctx.height {
				ir, ig, ib, ia := img.At(int(x), int(y)).RGBA()
				gr, gg, gb, ga := golden.At(int(x), int(y)).RGBA()

				abs := func(a int64) int64 {
					if a >= 0 {
						return a
					} else {
						return -a
					}
				}

				const maxDiff = 844
				dr := abs(int64(ir) - int64(gr))
				dg := abs(int64(ig) - int64(gg))
				db := abs(int64(ib) - int64(gb))
				da := abs(int64(ia) - int64(ga))

				if dr > maxDiff || dg > maxDiff || db > maxDiff || da > maxDiff {
					worstDiff = max(dr, dg, db, da, worstDiff)
					numWrong++
					failed = true
				}
			}
		}
	}

	if failed {
		if err := os.MkdirAll("./testdata/failed", 0777); err != nil {
			t.Fatal(err)
		}
		ff, err := os.Create("./testdata/failed/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer ff.Close()
		if err := png.Encode(ff, img); err != nil {
			t.Fatal(err)
		}

		if worstDiff > 0 {
			t.Errorf("%d pixels were wrong, by up to %d/65535", numWrong, worstDiff)
		}
		t.Fatalf("result (./testdata/failed/%[1]s.png) doesn't match golden file (./testdata/golden/%[1]s.png)", name)
	}
}

func TestIncorrectFilling1(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 8, 8, false, "incorrect_filling_1", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(4, 0))
		path.LineTo(curve.Pt(8, 4))
		path.LineTo(curve.Pt(4, 8))
		path.LineTo(curve.Pt(0, 4))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling2(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 64, 64, false, "incorrect_filling_2", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(16, 16))
		path.LineTo(curve.Pt(48, 16))
		path.LineTo(curve.Pt(48, 48))
		path.LineTo(curve.Pt(16, 48))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling3(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 9, 9, false, "incorrect_filling_3", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(4.00001, 1e-45))
		path.LineTo(curve.Pt(8.00001, 4.00001))
		path.LineTo(curve.Pt(4.00001, 8.00001))
		path.LineTo(curve.Pt(1e-45, 4.00001))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling4(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 64, 64, false, "incorrect_filling_4", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(16.000002, 8))
		path.LineTo(curve.Pt(20.000002, 8))
		path.LineTo(curve.Pt(24.000002, 8))
		path.LineTo(curve.Pt(28.000002, 8))
		path.LineTo(curve.Pt(32.000002, 8))
		path.LineTo(curve.Pt(32.000002, 9))
		path.LineTo(curve.Pt(28.000002, 9))
		path.LineTo(curve.Pt(24.000002, 9))
		path.LineTo(curve.Pt(20.000002, 9))
		path.LineTo(curve.Pt(16.000002, 9))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling5(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 32, 32, false, "incorrect_filling_5", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(16, 8))
		path.LineTo(curve.Pt(16, 9))
		path.LineTo(curve.Pt(32, 9))
		path.LineTo(curve.Pt(32, 8))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling6(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 32, 32, false, "incorrect_filling_6", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(16, 8))
		path.LineTo(curve.Pt(31.999998, 8))
		path.LineTo(curve.Pt(31.999998, 9))
		path.LineTo(curve.Pt(16, 9))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling7(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 32, 32, false, "incorrect_filling_7", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(32.000002, 9))
		path.LineTo(curve.Pt(28, 9))
		path.LineTo(curve.Pt(28, 8))
		path.LineTo(curve.Pt(32.000002, 8))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestIncorrectFilling8(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/2
	renderAndCompare(t, 32, 32, false, "incorrect_filling_8", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(16.000427, 8))
		path.LineTo(curve.Pt(20.000427, 8))
		path.LineTo(curve.Pt(24.000427, 8))
		path.LineTo(curve.Pt(28.000427, 8))
		path.LineTo(curve.Pt(32.000427, 8))
		path.LineTo(curve.Pt(32.000427, 9))
		path.LineTo(curve.Pt(28.000427, 9))
		path.LineTo(curve.Pt(24.000427, 9))
		path.LineTo(curve.Pt(20.000427, 9))
		path.LineTo(curve.Pt(16.000427, 9))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestOutOfBoundStrip(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/11
	ctx := getCtx(256, 256, true)
	var path curve.BezPath
	path.MoveTo(curve.Pt(256, 254))
	path.LineTo(curve.Pt(265, 254))
	ctx.Stroke(path, curve.Identity, curve.DefaultStroke.WithWidth(1), gfx.Solid(color.Make(color.SRGB, 1, 0, 0, 1)))
	// Test that we don't panic.
	render(ctx)
}

func starPath() curve.BezPath {
	var path curve.BezPath
	path.MoveTo(curve.Pt(50.0, 10.0))
	path.LineTo(curve.Pt(75.0, 90.0))
	path.LineTo(curve.Pt(10.0, 40.0))
	path.LineTo(curve.Pt(90.0, 40.0))
	path.LineTo(curve.Pt(25.0, 90.0))
	path.LineTo(curve.Pt(50.0, 10.0))
	return path
}

func TestFillingNonZeroRule(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "filling_nonzero_rule", func(ctx *Renderer) {
		star := starPath()
		ctx.Fill(star, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.5, 0, 0, 1)))
	})
}

func TestFillingEvenOddRule(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "filling_evenodd_rule", func(ctx *Renderer) {
		star := starPath()
		ctx.Fill(star, curve.Identity, gfx.EvenOdd, gfx.Solid(color.Make(color.SRGB, 0.5, 0, 0, 1)))
	})
}

func TestFillingUnclosedPath1(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/12
	renderAndCompare(t, 100, 100, false, "filling_unclosed_path_1", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(75, 25))
		path.LineTo(curve.Pt(25, 25))
		path.LineTo(curve.Pt(25, 75))
		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestFillingUnclosedPath2(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/12
	renderAndCompare(t, 100, 100, false, "filling_unclosed_path_2", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(50, 0))
		path.LineTo(curve.Pt(0, 0))
		path.LineTo(curve.Pt(0, 50))

		path.MoveTo(curve.Pt(50, 50))
		path.LineTo(curve.Pt(100, 50))
		path.LineTo(curve.Pt(100, 100))
		path.LineTo(curve.Pt(50, 100))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestTriangleExceedingViewport1(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/28
	renderAndCompare(t, 15, 8, false, "triangle_exceeding_viewport_1", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(5, 0))
		path.LineTo(curve.Pt(12, 7.99))
		path.LineTo(curve.Pt(-4, 7.99))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.EvenOdd, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestTriangleExceedingViewport2(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/28
	renderAndCompare(t, 15, 8, false, "triangle_exceeding_viewport_2", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(4, 0))
		path.LineTo(curve.Pt(11, 7.99))
		path.LineTo(curve.Pt(-5, 7.99))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.EvenOdd, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestShapeAtWideTileBoundary(t *testing.T) {
	t.Parallel()

	// https://github.com/LaurenzV/cpu-sparse-experiments/issues/30
	ctx := getCtx(256, 4, false)
	var path curve.BezPath
	path.MoveTo(curve.Pt(248, 0))
	path.LineTo(curve.Pt(257, 0))
	path.LineTo(curve.Pt(257, 2))
	path.LineTo(curve.Pt(248, 2))
	path.ClosePath()

	ctx.Fill(path, curve.Identity, gfx.EvenOdd, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	// Make sure we don't panic.
	render(ctx)
}

func TestFullCover1(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 8, 8, true, "full_cover_1", func(ctx *Renderer) {
		c := curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(8, 8))
		ctx.Fill(c, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.96, 0.96, 0.86, 1)))
	})
}

func TestFilledTriangle(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "filled_triangle", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(5, 5))
		path.LineTo(curve.Pt(95, 50))
		path.LineTo(curve.Pt(5, 95))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestStrokedTriangle(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "stroked_triangle", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(5, 5))
		path.LineTo(curve.Pt(95, 50))
		path.LineTo(curve.Pt(5, 95))
		path.ClosePath()

		ctx.Stroke(path, curve.Identity, curve.DefaultStroke.WithWidth(3), gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestFilledCircle(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "filled_circle", func(ctx *Renderer) {
		c := curve.Circle{
			Center: curve.Pt(50, 50),
			Radius: 45,
		}
		ctx.Fill(c, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestFilledCircleWithOpacity(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "filled_circle_with_opacity", func(ctx *Renderer) {
		c := curve.Circle{
			Center: curve.Pt(50, 50),
			Radius: 45,
		}
		ctx.Fill(c, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.4, 0.2, 0.6, 0.5)))
	})
}

func TestFilledOverlappingCircles(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "filled_overlapping_circles", func(ctx *Renderer) {
		for _, e := range []struct {
			x     float64
			y     float64
			color color.Color
		}{
			{35, 35, color.Make(color.SRGB, 1, 0, 0, 0.5)},
			{65, 35, color.Make(color.SRGB, 0, 0.5, 0, 0.5)},
			{50, 65, color.Make(color.SRGB, 0, 0, 1, 0.5)},
		} {
			circle := curve.Circle{Center: curve.Pt(e.x, e.y), Radius: 30}
			ctx.Fill(circle, curve.Identity, gfx.NonZero, gfx.Solid(e.color))
		}
	})
}

func TestStrokedCircle(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 100, 100, false, "stroked_circle", func(ctx *Renderer) {
		circle := curve.Circle{Center: curve.Pt(50, 50), Radius: 45}
		stroke := curve.DefaultStroke.WithWidth(3)

		ctx.Stroke(circle, curve.Identity, stroke, gfx.Solid(color.Make(color.SRGB, 0, 1, 0, 1)))
	})
}

func TestTriangleAboveAndWiderThanViewport(t *testing.T) {
	t.Parallel()

	// Requires winding of the first row of tiles to be calculcated correctly for sloped lines.
	renderAndCompare(t, 10, 10, false, "triangle_above_and_wider_than_viewport", func(ctx *Renderer) {
		var path curve.BezPath
		path.MoveTo(curve.Pt(5, -5))
		path.LineTo(curve.Pt(14, 6))
		path.LineTo(curve.Pt(-8, 6))
		path.ClosePath()

		ctx.Fill(path, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.7, 0.6, 0.8, 1)))
	})
}

func TestRectangleLeftOfViewport(t *testing.T) {
	t.Parallel()

	// Requires winding and pixel coverage to be calculcated correctly for tiles preceding the
	// viewport in scan direction.
	renderAndCompare(t, 10, 10, false, "rectangle_left_of_viewport", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(-4, 3), curve.Pt(1, 8))
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.7, 0.6, 0.8, 1)))
	})
}

func TestRectangleFlippedY(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 3, 2, true, "rectangle_flipped_y", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(-2, 3), curve.Pt(-1, -1))
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 1, 0, 0, 1)))
	})
}

func TestFilledAlignedRect(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 30, 20, false, "filled_aligned_rect", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(1, 1), curve.Pt(29, 19))
		ctx.Fill(rect, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.7, 0.6, 0.8, 1)))
	})
}

func TestStrokedUnalignedRect(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 30, 30, false, "stroked_unaligned_rect", func(ctx *Renderer) {
		rect := curve.NewRectFromPoints(curve.Pt(5, 5), curve.Pt(25, 25))
		stroke := curve.DefaultStroke.WithWidth(1).WithJoin(curve.MiterJoin)
		ctx.Stroke(rect, curve.Identity, stroke, gfx.Solid(color.Make(color.SRGB, 0.7, 0.6, 0.8, 1)))
	})
}

func TestClipping(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 64, 64, true, "clipping", func(ctx *Renderer) {
		var triangle curve.BezPath
		triangle.MoveTo(curve.Pt(2.0, 2.0))
		triangle.LineTo(curve.Pt(36.0, 4.0))
		triangle.LineTo(curve.Pt(6.0, 36.0))
		triangle.ClosePath()

		circle := curve.Circle{
			Center: curve.Pt(20.0, 20.0),
			Radius: 19.0,
		}

		ctx.Fill(curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(64, 64)), curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.8, 0, 0, 0.5)))

		ctx.PushClip(circle, curve.Identity, gfx.NonZero)
		ctx.PushClip(triangle, curve.Identity, gfx.NonZero)
		ctx.Fill(triangle, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 1, 1, 1, 0.5)))
		ctx.Stroke(circle, curve.Identity, curve.DefaultStroke.WithWidth(5), gfx.Solid(color.Make(color.SRGB, 0, 0, 1, 1)))
		ctx.Stroke(triangle, curve.Identity, curve.DefaultStroke.WithWidth(5), gfx.Solid(color.Make(color.SRGB, 0, 0, 1, 1)))
	})
}

func TestLinearAntiAliasing(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 32, 32, true, "linear_anti_aliasing", func(ctx *Renderer) {
		r := curve.NewRectFromOrigin(curve.Pt(16.5, 0), curve.Sz(1, 32))
		// We expect pixels 16 and 17 to be 50% transparent red.
		ctx.Fill(r, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.LinearSRGB, 1, 0, 0, 1)))
	})
}

func Test50pctGrey(t *testing.T) {
	t.Parallel()

	renderAndCompare(t, 32, 32, false, "50pct_grey", func(ctx *Renderer) {
		r := curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(32, 32))
		ctx.Fill(r, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.LinearSRGB, 0.5, 0.5, 0.5, 1)))
	})
}

// TestPixelsSliceBoundsCheck tests that Pixels.slice correctly rejects
// an end value that exceeds the sub-slice's width, even if it doesn't
// exceed p.end (the absolute end).
func TestPixelsSliceBoundsCheck(t *testing.T) {
	t.Parallel()

	var buf WideTileBuffer
	// Create a sub-slice: start=128, end=256, width=128
	p := buf.pixels(128, 128)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Pixels.slice(0, 200) should panic: end (200) exceeds width (128)")
		}
	}()
	// end=200 exceeds width=128 but not p.end=256, so the current
	// buggy check (end > p.end → 200 > 256 → false) lets it through.
	p.slice(0, 200)
}

func writeF32AsU16(in []gfx.PlainColor, out [][8]uint8) {
	_ = out[len(in)]
	for i := range in {
		px := in[i]

		// Un-premultiply, avoiding division by zero
		px[0] /= max(px[3], 1e-10)
		px[1] /= max(px[3], 1e-10)
		px[2] /= max(px[3], 1e-10)

		// Convert from linear sRGB to sRGB, gamut mapping
		c := color.Make(gfx.ColorSpace, float64(px[0]), float64(px[1]), float64(px[2]), 1)
		c = color.GamutMapCSS(c, color.SRGB)
		px[0] = float32(c.Values[0])
		px[1] = float32(c.Values[1])
		px[2] = float32(c.Values[2])

		// Scale to 16-bit
		px[0] *= 65535
		px[1] *= 65535
		px[2] *= 65535
		px[3] *= 65535

		binary.BigEndian.PutUint16(out[i][0:], uint16(px[0]+0.5))
		binary.BigEndian.PutUint16(out[i][2:], uint16(px[1]+0.5))
		binary.BigEndian.PutUint16(out[i][4:], uint16(px[2]+0.5))
		binary.BigEndian.PutUint16(out[i][6:], uint16(px[3]+0.5))
	}
}

func benchmarkFill(b *testing.B, fn func(b *testing.B, buf Pixels)) {
	var wtb WideTileBuffer
	buf := wtb.allPixels()

	// We test the full width to measure the best possible performance, and at
	// the smallest possible width to measure the per-call overhead.
	fillWidths := []int{wideTileWidth, 1}
	for _, width := range fillWidths {
		b.Run(fmt.Sprintf("width=%d", width), func(b *testing.B) {
			fn(b, buf)
			px := float64(width * tileHeight * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}
}

func Benchmark_fineFillComplexNative(b *testing.B) {
	c := gfx.PlainColor{0.5, 0.5, 0.5, 0.5}
	benchmarkFill(b, func(b *testing.B, buf Pixels) {
		for b.Loop() {
			fineFillComplexScalar(buf, c)
		}
	})
}

func Benchmark_memsetColumns(b *testing.B) {
	c := gfx.PlainColor{1, 1, 1, 1}
	benchmarkFill(b, func(b *testing.B, buf Pixels) {
		for b.Loop() {
			memsetColumns(buf, c)
		}
	})
}

func benchmarkFinePack(b *testing.B, complex bool) {
	tests := []struct {
		label  string
		width  uint16
		height uint16
	}{
		// 256*4 uses 16 KiB, which fits into L1 on somewhat modern CPUs.
		{"L1", 256, 4},
		// 256*128 uses 512 KiB, which fits into L2 on somewhat modern CPUs.
		{"L2", 256, 128},
		// 512*512 uses 4 MiB, which fits into L3 on somewhat modern CPUs.
		{"L3", 512, 512},
		// 4096*4096 uses 256 MiB, which does not fit into L3 on most CPUs.
		{"mem", 4096, 4096},
	}

	for _, tt := range tests {
		b.Run(tt.label, func(b *testing.B) {
			if tt.width%wideTileWidth != 0 {
				b.Fatalf("width %d isn't multiple of wideTileWidth", tt.width)
			}
			if tt.height%stripHeight != 0 {
				b.Fatalf("height %d isn't multiple of stripHeight", tt.height)
			}

			pixmap := make([]gfx.PlainColor, int(tt.width)*int(tt.height))
			packer := &PackerFloat32{
				Out:    pixmap,
				Width:  int(tt.width),
				Height: int(tt.height),
			}
			f := newFine(packer)
			clear(pixmap)
			if complex {
				scratch := f.layers[len(f.layers)-1].scratch
				for ch := range 4 {
					clear(scratch[ch][:])
				}
				f.layers[len(f.layers)-1].complex = true
			}

			for b.Loop() {
				for x := range tt.width / wideTileWidth {
					for y := range tt.height / stripHeight {
						f.setTile(nil, x, y)
						f.pack()
					}
				}
			}

			px := float64(int(tt.width) * int(tt.height) * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}
}

func Benchmark_fine_pack_simple(b *testing.B) {
	benchmarkFinePack(b, false)
}

func Benchmark_fine_pack_complex(b *testing.B) {
	benchmarkFinePack(b, true)
}
