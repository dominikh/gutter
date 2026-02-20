// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"math"
	"testing"

	"honnef.co/go/gutter/gfx"
)

var packUint8SrgbTests = []packUint8SrgbTest{
	{
		name:     "polynomial",
		instr:    "scalar",
		maxError: 0.5221,
		fn:       packUint8SRGB_Polynomial_Scalar,
	},

	{
		name:     "lut",
		instr:    "scalar",
		maxError: 0.545,
		fn:       packUint8SRGB_LUT_Scalar,
	},
}

type packUint8SrgbTest struct {
	name     string
	instr    string
	maxError float64
	fn       packUint8Fn
	disabled bool
}

func toSrgbF32(
	in *[wideTileWidth][stripHeight]gfx.PlainColor,
	out *[wideTileWidth][stripHeight]gfx.PlainColor,
	unpremul bool,
) {
	for x := range in {
		for y, px := range &in[x] {
			px[3] = max(px[3], 0.00001)
			if unpremul {
				px[0] /= px[3]
				px[1] /= px[3]
				px[2] /= px[3]
			}
			for k, v := range px[:3] {
				if v < 0.0031308 {
					out[x][y][k] = float32(12.92 * v * 255)
				} else {
					s := 1.055*(math.Pow(float64(v), 1.0/2.4)) - 0.055
					out[x][y][k] = float32(s * 255)
				}
			}
			out[x][y][3] = 255 * px[3]
		}
	}
}

func TestPackUint8Srgb(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	for _, tt := range packUint8SrgbTests {
		for _, unpremul := range []bool{false, true} {
			for _, dims := range []struct{ width, height int }{
				{wideTileWidth, stripHeight},
				// This will use SIMD for most of it, and scalar code for the
				// last 30 columns.
				{wideTileWidth - 2, stripHeight},
				// This will not use any SIMD because the height doesn't match
				// the strip height.
				{wideTileWidth, stripHeight - 2},
				// This will not use any SIMD because the height doesn't match
				// the strip height.
				{wideTileWidth - 2, stripHeight - 2},
			} {
				t.Run(fmt.Sprintf("name=%s/unpremul=%t/instr=%s/dims=%dx%d",
					tt.name, unpremul, tt.instr, dims.width, dims.height), func(t *testing.T) {

					if tt.disabled {
						t.SkipNow()
					}
					t.Parallel()
					if tt.maxError > 0.6 {
						t.Fatalf("specified max error (%g) violates DirectX requirements (<= 0.6)", tt.maxError)
					}
					const N = wideTileWidth * stripHeight
					var in WideTileBuffer
					out := make([][4]uint8, dims.width*dims.height)
					var outRef WideTileBuffer
					totalWronglyRoundedError := 0.0
					maxErr := 0.0
					numWronglyRounded := 0

					for v := int32(0); v <= 0x3f800000; v += N {
						for x := range in {
							for y := range in[x] {
								// We need wide gaps between neighboring inputs, otherwise
								// all inputs might map to the same output and all be within
								// the allowed tolerance, even if the implementation is
								// buggy.
								//
								// Because floating point values aren't evenly distributed, we
								// alternate between small and large values.
								//
								// For unpremul == false, this loop checks every
								// floating point value between 0 and 1, ensuring we're
								// staying under the max error and collecting
								// statistics. For unpremul == true, we may not
								// encounter every possible value, but we do ensure that
								// values get unpremultiplied correctly.
								v1 := max(min(v+int32(x*stripHeight+y), 0x3f800000), 0)
								v2 := max(min(0x3f800000-(v+int32(x*stripHeight+y)), 0x3f800000), 0)
								in[x][y] = gfx.PlainColor{
									math.Float32frombits(uint32(v1)),
									math.Float32frombits(uint32(v2)),
									math.Float32frombits(uint32(v1)),
									1,
								}
								if unpremul {
									// Premultiply values
									in[x][y][0] *= in[x][y][3]
									in[x][y][1] *= in[x][y][3]
									in[x][y][2] *= in[x][y][3]
								}
							}
						}
						tt.fn(
							&in,
							out,
							dims.width,
							dims.width,
							dims.height,
							unpremul,
						)
						toSrgbF32(&in, &outRef, unpremul)
						for y := range dims.height {
							for x := range dims.width {
								px := in[x][y]
								// in and outRef are stored in column major order,
								// while out is stored in row major order.
								approx := out[y*dims.width+x]
								ref := outRef[x][y]
								for k := range approx {
									e := math.Abs(float64(float32(approx[k]) - ref[k]))
									maxErr = max(maxErr, e)
									if uint8(0.5+ref[k]) != approx[k] {
										totalWronglyRoundedError += e
										numWronglyRounded++
										// log.Printf("mismatch: for %g, ref=%g, approx=%d", px[k], ref[k], approx[k])
									}
									if !(e <= tt.maxError) {
										t.Fatalf("ref(%v) = %g, f(...) = %v, error = %g > %g",
											px, ref, approx, e, tt.maxError)
									}
								}
							}
						}
					}
					totalChecks := float32(0x3f800001) * 3
					t.Logf("num wrongly rounded: %d (%f%%)",
						numWronglyRounded,
						100*float32(numWronglyRounded)/totalChecks)
					t.Log("max error:", maxErr)
					t.Log("average error of wrongly rounded:",
						totalWronglyRoundedError/float64(numWronglyRounded))
				})
			}
		}
	}
}

func BenchmarkPackUint8Srgb(b *testing.B) {
	ins := map[bool]*WideTileBuffer{
		false: new(WideTileBuffer),
		true:  new(WideTileBuffer),
	}
	for x := range wideTileWidth {
		for y := range stripHeight {
			in := ins[false]
			in[x][y] = gfx.PlainColor{
				float32((x*stripHeight+y)%11) * 0.00001,
				float32((x*stripHeight+y)%11) * 0.1,
				float32((x*stripHeight+y)%11) * 0.00001,
				float32((x*stripHeight+y)%11) * 0.1,
			}
		}
	}
	copy(ins[true][:], ins[false][:])
	for x := range wideTileWidth {
		for y := range stripHeight {
			// Premultiply values
			ins[true][x][y][0] *= ins[true][x][y][3]
			ins[true][x][y][1] *= ins[true][x][y][3]
			ins[true][x][y][2] *= ins[true][x][y][3]
		}
	}
	out := make([][4]uint8, wideTileWidth*stripHeight)
	for _, tt := range packUint8SrgbTests {
		for _, unpremul := range []bool{false, true} {
			in := ins[unpremul]
			b.Run(fmt.Sprintf("name=%s/unpremul=%t/instr=%s", tt.name, unpremul, tt.instr), func(b *testing.B) {
				if tt.disabled {
					b.SkipNow()
				}

				for b.Loop() {
					tt.fn(
						in,
						out,
						wideTileWidth,
						wideTileWidth,
						stripHeight,
						unpremul,
					)
				}

				b.ReportMetric(float64(b.Elapsed())/float64(wideTileWidth*stripHeight*b.N), "ns/px")
			})
		}
	}
}
