// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package fontdb

import (
	"cmp"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"honnef.co/go/gutter/opentype"
	"honnef.co/go/gutter/opentype/opentypehl"
	"honnef.co/go/stuff/syncutil"

	"github.com/go-text/typesetting/fontscan"
	"golang.org/x/text/language"
)

type Faces struct {
	// All fonts grouped by typographic family name
	Faces map[string][]*Face
}

type Face struct {
	Path string
	Axes map[opentype.Tag]Axis
}

type Axis struct {
	Tag               opentype.Tag
	Min, Default, Max float64
}

type FontVariation struct {
	Font       *Face
	AxisValues map[opentype.Tag]float64
}

func New() *Faces {
	type font struct {
		family string
		font   *Face
	}

	en := language.MustParseBase("en")
	ret, _ := dmap(fontFiles(), 8, nil, func(subitems []string) ([]font, error) {
		var out []font
		for _, f := range subitems {
			func() {
				if strings.HasSuffix(f, ".ttc") {
					// XXX support collections
					return
				}

				fd, err := os.Open(f)
				if err != nil {
					slog.Info("couldn't open font", "file", f, "err", err)
					return
				}
				defer fd.Close()
				off, err := fd.Seek(0, io.SeekEnd)
				if err != nil {
					slog.Info("couldn't seek font", "file", f, "err", err)
					return
				}
				data, err := syscall.Mmap(int(fd.Fd()), 0, int(off), syscall.PROT_READ, syscall.MAP_SHARED)
				if err != nil {
					slog.Info("couldn't mmap font", "file", f, "err", err)
					return
				}
				fd.Close()

				fnt, err := opentypehl.NewFile(data)
				if err != nil {
					slog.Info("couldn't parse font", "file", f, "err", err)
					return
				}

				var fam string
			nameLoop:
				for name := range fnt.Names().All() {
					switch name.ID() {
					case opentype.NameTypographicFamilyName:
					case opentype.NameWWSFamilyName:
					case opentype.NameFontFamilyName:
					default:
						continue
					}

					// TODO(dh): collect family names in all languages
					if lang, _ := name.Language().Base(); lang != en {
						continue
					}
					switch name.ID() {
					case opentype.NameTypographicFamilyName:
						fam = name.String()
						break nameLoop
					case opentype.NameWWSFamilyName:
						fam = name.String()
					case opentype.NameFontFamilyName:
						if fam == "" {
							fam = name.String()
						}
					}
				}

				axes := make(map[opentype.Tag]Axis)
				os2Raw, ok := fnt.Raw().FindTable("OS/2")
				if !ok {
					return
				}
				var os2 opentype.OS2Table
				opentype.ParseOS2Table(os2Raw.Data(), &os2)

				wght := float64(os2.UsWeightClass)
				axes["wght"] = Axis{"wght", wght, wght, wght}

				var wdth float64
				switch os2.UsWidthClass {
				case opentype.WidthUltraCondensed:
					wdth = 50
				case opentype.WidthExtraCondensed:
					wdth = 62.5
				case opentype.WidthCondensed:
					wdth = 75
				case opentype.WidthSemiCondensed:
					wdth = 87.5
				case opentype.WidthMedium:
					wdth = 100
				case opentype.WidthSemiExpanded:
					wdth = 112.5
				case opentype.WidthExpanded:
					wdth = 125
				case opentype.WidthExtraExpanded:
					wdth = 150
				case opentype.WidthUltraExpanded:
					wdth = 200
				default:
					// Invalid width class, fall back to default
					wdth = 100
				}
				axes["wdth"] = Axis{"wdth", wdth, wdth, wdth}

				fss := os2.FsSelection
				if fss&opentype.FsItalic != 0 {
					axes["ital"] = Axis{"ital", 1, 1, 1}
				} else {
					axes["ital"] = Axis{"ital", 0, 0, 0}
				}

				// Overwrite computed axes with any real axes provided by the font
				if fvarRaw, ok := fnt.Raw().FindTable("fvar"); ok {
					var fvar opentype.FvarTable
					opentype.ParseFvarTable(fvarRaw.Data(), &fvar)
					for _, axis := range fvar.Axes() {
						axes[axis.Tag] = Axis{
							axis.Tag,
							axis.MinValue.Float(),
							axis.DefaultValue.Float(),
							axis.MaxValue.Float(),
						}
					}
				}

				out = append(out, font{fam, &Face{
					Path: f,
					Axes: axes,
				}})
			}()
		}
		return out, nil
	})

	fonts := make(map[string][]*Face)
	for _, grp := range ret {
		for _, font := range grp {
			fonts[font.family] = append(fonts[font.family], font.font)
		}
	}

	if len(fonts) == 0 {
		slog.Warn("couldn't find any fonts")
	}

	return &Faces{Faces: fonts}
}

type nilLogger struct{}

// Printf implements fontscan.Logger.
func (n nilLogger) Printf(format string, args ...interface{}) {}

// fontFiles collects all discoverable font files.
func fontFiles() []string {
	// The error returned by DefaultFontDirectories it useless, it only tells us
	// that it couldn't find any directories.
	dirs, _ := fontscan.DefaultFontDirectories(nilLogger{})

	const p = 4
	outs, _ := dmap(dirs, p, nil, func(subdirs []string) ([]string, error) {
		var out []string
		for _, dir := range subdirs {
			out = scanDir(dir, out)
		}
		return out, nil
	})

	numOuts := 0
	for _, out := range outs {
		numOuts += len(out)
	}
	all := make([]string, 0, numOuts)
	for _, out := range outs {
		all = append(all, out...)
	}
	return all
}

func scanDir(dir string, out []string) []string {
	dents, err := os.ReadDir(dir)
	if err != nil {
		slog.Info("couldn't read font directory", "err", err, "dir", dir)
		return out
	}
	for _, dent := range dents {
		path := filepath.Join(dir, dent.Name())
		switch dent.Type() {
		case fs.ModeDir:
			out = scanDir(path, out)
			continue
		case fs.ModeSymlink:
			info, err := os.Stat(path)
			if err != nil {
				slog.Info("couldn't stat file", "err", err, "file", path)
				continue
			}

			// We avoid evaluating the symlink for the common cases: symlinks to
			// directories and symlinks to files (as opposed to symlinks to more
			// symlinks). This avoids some syscalls.
			switch typ := info.Mode().Type(); typ {
			case fs.ModeDir:
				out = scanDir(path, out)
				continue
			case fs.ModeSymlink:
				// Symlink to a symlink. At this point we recursively follow the
				// symlink.
				dst, err := filepath.EvalSymlinks(path)
				if err != nil {
					slog.Debug("skipping unevaluatable symlink", "err", err, "file", filepath.Join(dir, dent.Name()))
					continue
				}
				info, err := os.Stat(dst)
				if err != nil {
					slog.Info("couldn't stat file", "err", err, "file", dst)
					continue
				}
				typ := info.Mode().Type()
				switch typ {
				case fs.ModeDir:
					out = scanDir(dst, out)
					continue
				case fs.ModeSymlink:
					panic("unreachable")
				case 0:
					path = dst
				default:
					slog.Debug("skipping irregular file", "type", typ, "file", dst)
				}
			case 0:
			default:
				slog.Debug("skipping irregular file", "type", typ, "file", path)
			}
		case 0:
		default:
			slog.Debug("skipping irregular file", "type", dent.Type(), "file", filepath.Join(dir, dent.Name()))
		}

		switch filepath.Ext(path) {
		case ".ttf", ".otf", ".ttc", ".otc":
		default:
			continue
		}

		out = append(out, path)
	}

	return out
}

func dmap[S ~[]E, E, R any](items S, limit int, out []R, fn func(subitems S) (R, error)) ([]R, error) {
	if len(items) == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = runtime.GOMAXPROCS(0)
	}

	if limit > len(items) {
		limit = len(items)
	}

	out = slices.Grow(out, limit)[:len(out)+limit]
	err := syncutil.Distribute(items, limit, func(group int, step int, subitems S) error {
		res, err := fn(subitems)
		out[group] = res
		return err
	})

	return out, err
}

func (f *Faces) Match(family string, query map[opentype.Tag]float64) (*FontVariation, bool) {
	candidates := f.Faces[family]
	if len(candidates) == 0 {
		return nil, false
	}

	type distance struct {
		idx      int
		distance float64
		dot      float64
	}
	distances := make([]distance, len(candidates))
	for i, candidate := range candidates {
		dist, dot := vectorDistance(query, candidate.Axes)
		distances[i] = distance{i, dist, dot}
	}
	slices.SortStableFunc(distances, func(a, b distance) int {
		if c := cmp.Compare(a.distance, b.distance); c != 0 {
			return c
		}
		return cmp.Compare(a.dot, b.dot)
	})
	font := candidates[distances[0].idx]
	v := &FontVariation{
		Font:       font,
		AxisValues: make(map[opentype.Tag]float64),
	}
	for axis, rng := range font.Axes {
		if rng.Min == rng.Max {
			v.AxisValues[axis] = rng.Min
		}
		if qv, ok := query[axis]; ok {
			if qv >= rng.Min && qv <= rng.Max {
				v.AxisValues[axis] = qv
			} else if qv <= rng.Min {
				v.AxisValues[axis] = rng.Min
			} else {
				v.AxisValues[axis] = rng.Max
			}
		} else {
			v.AxisValues[axis] = rng.Default
		}
	}
	return v, true
}

func vectorDistance(query map[opentype.Tag]float64, candidate map[opentype.Tag]Axis) (dist, dot float64) {
	// TODO(dh): allow non-zero slnt to satisfy ital?

	seen := make(map[opentype.Tag]struct{})
	var sum float64
	do := func(k opentype.Tag) {
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		var standard, multiplier float64
		switch k {
		case "wght":
			multiplier = 5
			standard = 400
		case "wdth":
			multiplier = 55
			standard = 100
		case "ital":
			multiplier = 1400
			standard = 0
		case "slnt":
			multiplier = 35
			standard = 0
		case "opsz":
			multiplier = 1
			standard = 12
		default:
			multiplier = 1
			standard = 0
		}
		qv, ok := query[k]
		if !ok {
			qv = standard
		}
		var cv float64
		cva, ok := candidate[k]
		if ok {
			if qv >= cva.Min && qv <= cva.Max {
				cv = qv
			} else if qv <= cva.Min {
				cv = cva.Min
			} else {
				cv = cva.Max
			}
		} else {
			cv = standard
		}

		dot += qv * cv
		d := ((qv - standard) * multiplier) - ((cv - standard) * multiplier)
		sum += d * d
	}
	for k := range query {
		do(k)
	}
	for k := range candidate {
		do(k)
	}

	dist = math.Sqrt(sum)
	return dist, dot
}
