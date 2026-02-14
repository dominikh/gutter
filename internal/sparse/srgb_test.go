// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"math"
	"testing"
)

var linearRgbaF32ToSrgbU8Tests = []srgbTest{
	{
		name:     "reference",
		instr:    "scalar",
		maxError: 0.5,
		fn:       linearRgbaF32ToSrgbU8_Reference,
	},

	{
		name:     "polynomial",
		instr:    "scalar",
		maxError: 0.5221,
		fn:       linearRgbaF32ToSrgbU8_Polynomial_Scalar,
	},

	{
		name:     "lut",
		instr:    "scalar",
		maxError: 0.545,
		fn:       linearRgbaF32ToSrgbU8_LUT_Scalar,
	},
}

type srgbTest struct {
	name     string
	instr    string
	maxError float64
	fn       func(in [][4]float32, out [][4]uint8, unpremul bool)
	disabled bool
}

func linearRgbaF32ToSrgbU8_Reference(in [][4]float32, out [][4]uint8, unpremul bool) {
	for i, px := range in {
		px[3] = max(px[3], 0.00001)
		if unpremul {
			px[0] /= px[3]
			px[1] /= px[3]
			px[2] /= px[3]
		}
		for k, v := range px[:3] {
			if v < 0.0031308 {
				out[i][k] = uint8(0.5 + (12.92 * v * 255))
			} else {
				s := 1.055*(math.Pow(float64(v), 1.0/2.4)) - 0.055
				out[i][k] = uint8(0.5 + s*255)
			}
		}
		out[i][3] = uint8(0.5 + 255*px[3])
	}
}

func linearRgbaF32ToSrgbF32Reference(in [][4]float32, out [][4]float32, unpremul bool) {
	for i, px := range in {
		px[3] = max(px[3], 0.00001)
		if unpremul {
			px[0] /= px[3]
			px[1] /= px[3]
			px[2] /= px[3]
		}
		for k, v := range px[:3] {
			if v < 0.0031308 {
				out[i][k] = float32(12.92 * v * 255)
			} else {
				s := 1.055*(math.Pow(float64(v), 1.0/2.4)) - 0.055
				out[i][k] = float32(s * 255)
			}
		}
		out[i][3] = 255 * px[3]
	}
}

func TestLinearRgbaF32ToSrgbU8(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	for _, tt := range linearRgbaF32ToSrgbU8Tests {
		for _, unpremul := range []bool{false, true} {
			t.Run(fmt.Sprintf("name=%s/unpremul=%t/instr=%s", tt.name, unpremul, tt.instr), func(t *testing.T) {
				if tt.disabled {
					t.SkipNow()
				}
				t.Parallel()
				if tt.maxError > 0.6 {
					t.Fatalf("specified max error (%g) violates DirectX requirements (<= 0.6)", tt.maxError)
				}
				const N = 32
				in := make([][4]float32, N)
				out := make([][4]uint8, N)
				outRef := make([][4]float32, N)
				totalWronglyRoundedError := 0.0
				maxErr := 0.0
				numWronglyRounded := 0

				for v := int32(0); v <= 0x3f800000; v += N {
					for j := range in {
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
						v1 := max(min(v+int32(j), 0x3f800000), 0)
						v2 := max(min(0x3f800000-(v+int32(j)), 0x3f800000), 0)
						in[j] = [4]float32{
							math.Float32frombits(uint32(v1)),
							math.Float32frombits(uint32(v2)),
							math.Float32frombits(uint32(v1)),
							1,
						}
						if unpremul {
							// Premultiply values
							in[j][0] *= in[j][3]
							in[j][1] *= in[j][3]
							in[j][2] *= in[j][3]
						}
					}
					tt.fn(in, out, unpremul)
					linearRgbaF32ToSrgbF32Reference(in, outRef, unpremul)
					for j := range len(out) {
						px := in[j]
						approx := out[j]
						ref := outRef[j]
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

func BenchmarkLinearRgbaF32ToSrgbU8(b *testing.B) {
	const N = 1024
	ins := map[bool][][4]float32{
		false: make([][4]float32, N),
		true:  make([][4]float32, N),
	}
	for i := range N {
		in := ins[false]
		in[i] = [4]float32{
			float32(i%11) * 0.00001,
			float32(i%11) * 0.1,
			float32(i%11) * 0.00001,
			float32(i%11) * 0.1,
		}
	}
	copy(ins[true], ins[false])
	for i := range ins[true] {
		// Premultiply values
		ins[true][i][0] *= ins[true][i][3]
		ins[true][i][1] *= ins[true][i][3]
		ins[true][i][2] *= ins[true][i][3]
	}
	out := make([][4]uint8, N)
	for _, tt := range linearRgbaF32ToSrgbU8Tests {
		for _, unpremul := range []bool{false, true} {
			in := ins[unpremul]
			b.Run(fmt.Sprintf("name=%s/unpremul=%t/instr=%s", tt.name, unpremul, tt.instr), func(b *testing.B) {
				if tt.disabled {
					b.SkipNow()
				}

				for n := 32 / 4; n <= len(in); n *= 2 {
					b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
						for b.Loop() {
							for i := 0; i < len(in); i += n {
								tt.fn(in[i:][:n], out[i:][:n], unpremul)
							}
						}

						b.ReportMetric(float64(b.Elapsed())/float64(len(in)*b.N), "ns/px")
					})
				}
			})
		}
	}
}
