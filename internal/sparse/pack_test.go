// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"math"
	"testing"
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
	in *WideTileBuffer,
	out *WideTileBuffer,
	unpremul bool,
) {
	for x := range wideTileWidth {
		for y := range stripHeight {
			r, g, b, a := in[0][x][y], in[1][x][y], in[2][x][y], in[3][x][y]
			a = max(a, 0.00001)
			if unpremul {
				r /= a
				g /= a
				b /= a
			}
			for k, v := range [3]float32{r, g, b} {
				if v < 0.0031308 {
					out[k][x][y] = float32(12.92 * float64(v) * 255)
				} else {
					s := 1.055*(math.Pow(float64(v), 1.0/2.4)) - 0.055
					out[k][x][y] = float32(s * 255)
				}
			}
			out[3][x][y] = 255 * a
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
						for x := range wideTileWidth {
							for y := range stripHeight {
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
								r := math.Float32frombits(uint32(v1))
								g := math.Float32frombits(uint32(v2))
								b := math.Float32frombits(uint32(v1))
								a := float32(1)
								if unpremul {
									// Premultiply values
									r *= a
									g *= a
									b *= a
								}
								in[0][x][y] = r
								in[1][x][y] = g
								in[2][x][y] = b
								in[3][x][y] = a
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
								px := [4]float32{in[0][x][y], in[1][x][y], in[2][x][y], in[3][x][y]}
								// in and outRef are stored in column major order,
								// while out is stored in row major order.
								approx := out[y*dims.width+x]
								ref := [4]float32{outRef[0][x][y], outRef[1][x][y], outRef[2][x][y], outRef[3][x][y]}
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
			val := float32((x*stripHeight + y) % 11)
			in := ins[false]
			in[0][x][y] = val * 0.00001
			in[1][x][y] = val * 0.1
			in[2][x][y] = val * 0.00001
			in[3][x][y] = val * 0.1
		}
	}
	*ins[true] = *ins[false]
	for x := range wideTileWidth {
		for y := range stripHeight {
			// Premultiply values
			a := ins[true][3][x][y]
			ins[true][0][x][y] *= a
			ins[true][1][x][y] *= a
			ins[true][2][x][y] *= a
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
