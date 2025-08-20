// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package text

// TODO figure out what to do about optical size
// TODO figure out what to do about mirrored glyphs. harfbuzz? manually? font feature?
// TODO lerping of font features and variable axes, for animation purposes
// FIXME figure out alphabetic vs ideographic baselines
// TODO figure out leading distribution
// TODO add support for placeholders
// TODO leading trim (https://medium.com/microsoft-design/leading-trim-the-future-of-digital-typesetting-d082d84b202 / https://github.com/flutter/flutter/issues/146860)
// TODO https://www.figma.com/blog/line-height-changes/
// TODO https://aresluna.org/line-height-playground/
// TODO effect of emoji on line height
// TODO rendering color fonts
// TODO rendering bitmaps
// TODO allow specifying a font by file and index, for debugging
// TODO with the current split of ParagraphBuilder and Paragraph, changing any
//   style requires building a new Paragraph from scratch. That means that
//   merely changing the color of text will go through all the steps of bidi, font
//   fallback, shaping, line breaking, etc.

// # Layers of text
//
// A ParagraphBuilder is used to push/pop styles and add text with the currently
// active style. ParagraphBuilder splits the added text into runs by bidi,
// script, and used font (in addition to text already being split by Style). It
// then shapes all runs and returns a Paragraph.
//
// ParagraphBuilder.splitByFont uses fontdb.Faces and FontLoader to look up font
// families and create fonts.
//
// Paragraph splits the runs into multiple lines based on the max width, answers
// queries about metrics, and knows how to paint the runs into a gfx.Recorder.
//
// TextPainter is a RenderObject (FIXME: or is it?) that represents text as a
// tree of InlineSpan (which may be TextSpans or placeholders). It uses
// ParagraphBuilder to build a Paragraph

// # Line height
//
// On Windows, os2.usWinAscent and os2.usWinDescent describe the font's ascent
// and descent as used for clipping when rendering glyphs in Windows's GDI. Some
// legacy applications (mis)used these values to determine line height.
// os2.sTypoAscender and os2.sTypoDescender should be used for line height
// instead, but some applications require the USE_TYPO_METRICS bit in
// os2.fsSelection to be set to actually use the typographic ascenders.
//
// macOS uses hhea.Ascender and hhea.Descender for line height instead.
//
// os2.sTypoLineGap and hhea.LineGap specify additional leading for lines, used
// by Windows and macOS respectively. Different environments apply this leading
// differently, with web browsers famously spreading it across the top and
// bottom of a line.
//
// DTP applications such as InDesign set line height to 120% of the font size
// (e.g. 12pt height for 10pt font size), ignoring the line height set by the
// font.
//
// "On recent Apple systems, the typo values will be preferred over the hhea
// values if that font both contains a STAT table and Use Typo Metrics is turned
// on" (https://glyphsapp.com/learn/vertical-metrics)

// Indicates that if input text is changed on one side of the beginning of the
// cluster this glyph is part of, then the shaping results for the other side might
// change. Note that the absence of this flag will NOT by itself mean that it IS
// safe to concat text. Only two pieces of text both of which clear of this flag
// can be concatenated safely.

// This can be used to optimize paragraph layout, by avoiding re-shaping of each
// line after line-breaking, by limiting the reshaping to a small piece around the
// breaking position only, even if the breaking position carries the
// HarfBuzz.glyph_flags_t.UNSAFE_TO_BREAK or when hyphenation or other text
// transformation happens at line-break position, in the following way:

// 1. Iterate back from the line-break position until the first cluster start
// position that is NOT unsafe-to-concat,

// 2. shape the segment from there till the end of line,

// 3. check whether the resulting glyph-run also is clear of the unsafe-to-concat
// at its start-of-text position; if it is, just splice it into place and the line
// is shaped; If not, move on to a position further back that is clear of
// unsafe-to-concat and retry from there, and repeat. At the start of next line a
// similar algorithm can be implemented. That is:

// a. Iterate forward from the line-break position until the first cluster start
// position that is NOT unsafe-to-concat,

// b. shape the segment from beginning of the line to that position,

// c. check whether the resulting glyph-run also is clear of the unsafe-to-concat
// at its end-of-text position; if it is, just splice it into place and the
// beginning is shaped; If not, move on to a position further forward that is clear
// of unsafe-to-concat and retry up to there, and repeat.

// A slight complication will arise in the implementation of the algorithm above,
// because while our buffer API has a way to return flags for position
// corresponding to start-of-text, there is currently no position corresponding to
// end-of-text. This limitation can be alleviated by shaping more text than needed
// and looking for unsafe-to-concat flag within text clusters. The
// HarfBuzz.glyph_flags_t.UNSAFE_TO_BREAK flag will always imply this flag. To use
// this flag, you must enable the buffer flag
// HarfBuzz.buffer_flags_t.PRODUCE_UNSAFE_TO_CONCAT during shaping, otherwise the
// buffer flag will not be reliably produced.

import (
	"encoding/binary"
	"fmt"
	"image"
	"iter"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/fontdb"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/harfbuzz"
	xlanguage "honnef.co/go/gutter/internal/language"
	"honnef.co/go/gutter/opentype"
	"honnef.co/go/gutter/text/bidi"
	"honnef.co/go/gutter/text/linebreak"
	"honnef.co/go/stuff/container/maybe"
	"honnef.co/go/stuff/container/tinylfu"
)

// XXX make this a dynamic setting
const debugText = false

type FontStyle int

const (
	FontStyleNormal FontStyle = iota
	FontStyleItalic
)

type Alignment int

const (
	AlignmentStart = iota
	AlignmentEnd
	AlignmentLeft
	AlignmentRight
	AlignmentCenter
	AlignmentJustify
)

type Overflow int

const (
	OverflowClip Overflow = iota
	OverflowEllipsis
	OverflowVisible
)

type LeadingDistribution int

const (
	Proportional LeadingDistribution = iota
	Even
)

type Decoration int

const (
	DecorationLineThrough Decoration = 1 << iota
	DecorationOverline
	DecorationUnderline
)

type DecorationStyle int

const (
	DecorationSolid DecorationStyle = iota
	DecorationDouble
	DecorationDotted
	DecorationDashed
	DecorationWavy
)

type Face struct {
	Path  string
	Index int
}

type FontLoader struct {
	files map[string]*harfbuzz.Blob
	faces map[Face]*harfbuzz.Face
}

// XXX
//
// The returned font must be manually destroyed to free its resources.
func (fl *FontLoader) Font(face Face, vars map[opentype.Tag]float64) *harfbuzz.Font {
	hbface, ok := fl.faces[face]
	if !ok {
		blob, ok := fl.files[face.Path]
		if !ok {
			blob = harfbuzz.NewBlobFromFile(face.Path)
			if fl.files == nil {
				fl.files = make(map[string]*harfbuzz.Blob)
			}
			fl.files[face.Path] = blob
		}
		hbface = harfbuzz.NewFace(blob, face.Index)
		if fl.faces == nil {
			fl.faces = make(map[Face]*harfbuzz.Face)
		}
		fl.faces[face] = hbface
	}

	font := harfbuzz.NewFont(hbface)
	values := make([]harfbuzz.Variation, 0, len(vars))
	for k, v := range vars {
		values = append(values, harfbuzz.Variation{Tag: k, Value: v})
	}
	font.SetVariations(values)
	return font
}

func MakeDefaultStyle() *Style {
	return &Style{
		Fill:         maybe.Some(color.Make(color.SRGB, 0, 0, 0, 1)),
		FontSize:     maybe.Some(12.0),
		FontStyle:    maybe.Some(FontStyleNormal),
		FontFamilies: maybe.Some([]string{"generic(system-ui)"}),
	}
}

type Stroke struct {
	Style curve.Stroke
	Color color.Color
}

type FontFeature struct {
	Feature opentype.Tag
	Value   uint32
}

const (
	FontWeight opentype.Tag = "wght"
	FontWidth  opentype.Tag = "wdth"
)

type Style struct {
	// XXX textbaseline
	// XXX fontVariations

	// XXX merge stretch (width), style, and weight into variations
	DontInherit  bool
	Fill         maybe.Option[color.Color]
	Stroke       maybe.Option[Stroke]
	Background   maybe.Option[gfx.Paint]
	FontFamilies maybe.Option[[]string]

	// The font size, in logical pixels.
	//
	// The font size specifies the number of logical pixels in one em.
	FontSize   maybe.Option[float64]
	FontWeight maybe.Option[float64]
	FontWidth  maybe.Option[float64]
	FontStyle  maybe.Option[FontStyle]
	// The OpenType font features and their values to apply to the text span.
	// Unless DontInherit is also set, this value will be appended to the old
	// style's list of font features. To explicitly disable a feature, set its
	// value to zero.
	FontFeatures        maybe.Option[[]FontFeature]
	LetterSpacing       maybe.Option[float64]
	WordSpacing         maybe.Option[float64]
	LineHeight          maybe.Option[float64]
	LeadingDistribution maybe.Option[LeadingDistribution]
	Language            maybe.Option[xlanguage.Tag]
	Decoration          maybe.Option[Decoration]
	DecorationBrush     maybe.Option[gfx.Paint]
	DecorationStyle     maybe.Option[DecorationStyle]
	Overflow            maybe.Option[Overflow]
}

type runStyle struct {
	FontFamilies maybe.Option[[]string]
	FontSize     maybe.Option[float64]
	FontWeight   maybe.Option[float64]
	FontWidth    maybe.Option[float64]
	FontStyle    maybe.Option[FontStyle]
	Language     maybe.Option[xlanguage.Tag]
}

func (r *runStyle) Equal(o *runStyle) bool {
	if slices.Equal(r.FontFamilies.UnwrapOr(nil), o.FontFamilies.UnwrapOr(nil)) &&
		r.FontSize == o.FontSize &&
		r.FontWeight == o.FontWeight &&
		r.FontWidth == o.FontWidth &&
		r.FontStyle == o.FontStyle &&
		r.Language == o.Language {

		return true
	}
	return false
}

// Merge returns a style with attributes in ts that are also set in ts2 merged
// with those from ts2. If ts2 is nil, Merge returns ts. If ts is nil, Merge
// returns ts2. If ts2.DontInherit is set, Merge returns ts2. In all other
// cases, Merge returns a new style.
func (ts *Style) Merge(ts2 *Style) *Style {
	if ts == nil {
		return ts2
	}
	if ts2 == nil {
		return ts
	}
	if ts2.DontInherit {
		return ts2
	}
	out := *ts
	out.DontInherit = false
	if ts2.Fill.Set() {
		out.Fill = ts2.Fill
	}
	if ts2.Stroke.Set() {
		out.Stroke = ts2.Stroke
	}
	if ts2.Background.Set() {
		out.Background = ts2.Background
	}
	if ts2.FontSize.Set() {
		out.FontSize = ts2.FontSize
	}
	if ts2.FontWeight.Set() {
		out.FontWeight = ts2.FontWeight
	}
	if ts2.FontWidth.Set() {
		out.FontWidth = ts2.FontWidth
	}
	if ts2.FontStyle.Set() {
		out.FontStyle = ts2.FontStyle
	}
	if features, ok := ts2.FontFeatures.Get(); ok {
		old, _ := out.FontFeatures.Get()
		n := len(old)
		out.FontFeatures = maybe.Some(append(old[:n:n], features...))
	}
	if ts2.LetterSpacing.Set() {
		out.LetterSpacing = ts2.LetterSpacing
	}
	if ts2.WordSpacing.Set() {
		out.WordSpacing = ts2.WordSpacing
	}
	if ts2.LineHeight.Set() {
		out.LineHeight = ts2.LineHeight
	}
	if ts2.LeadingDistribution.Set() {
		out.LeadingDistribution = ts2.LeadingDistribution
	}
	if ts2.Language.Set() {
		out.Language = ts2.Language
	}
	if ts2.Decoration.Set() {
		out.Decoration = ts2.Decoration
	}
	if ts2.DecorationBrush.Set() {
		out.DecorationBrush = ts2.DecorationBrush
	}
	if ts2.DecorationStyle.Set() {
		out.DecorationStyle = ts2.DecorationStyle
	}
	if ts2.FontFamilies.Set() {
		out.FontFamilies = ts2.FontFamilies
	}
	if ts2.Overflow.Set() {
		out.Overflow = ts2.Overflow
	}
	return &out
}

type HeightBehavior struct {
	LeadingDistribution LeadingDistribution
	ApplyToFirstAscent  bool
	ApplyToLastDescent  bool
}

type ParagraphStyle struct {
	Alignment          Alignment
	Direction          bidi.Direction
	MaxLines           int
	TextHeightBehavior HeightBehavior
	// StrutStyle // XXX
	Ellipsis string
}

type Paragraph struct {
	style *ParagraphStyle
	text  text
	lines []line

	infos   [][]harfbuzz.GlyphInfo
	poss    [][]harfbuzz.GlyphPosition
	extents [][]harfbuzz.GlyphExtents
	lb      linebreak.Result

	// OPT(dh): this only caches recordings, not sparse strips.
	filledGlyphCache map[filledGlyphCacheKey]gfx.Recording
}

type filledGlyphCacheKey struct {
	glyph int32
	font  *Font
	color color.Color
}

// func (p *Paragraph) Width() float64               {}
// func (p *Paragraph) Height() float64              {}
// func (p *Paragraph) ExceededMaxLines() bool       {}
// func (p *Paragraph) AlphabeticBaseline() float64  {}
// func (p *Paragraph) IdeographicBaseline() float64 {}
// func (p *Paragraph) LongestLine() float64         {}

func (p *Paragraph) NumLines() int {
	return len(p.lines)
}

// func (p *Paragraph) LineMetrics() []LineMetrics {}

// func (p *Paragraph) PlaceholderBoxes() []TextBox  {}

type line struct {
	start, end int
	width      float64
	runs       []run
}

func (p *Paragraph) shapeRuns(buf *harfbuzz.Buffer, runs []run) ([][]harfbuzz.GlyphInfo, [][]harfbuzz.GlyphPosition) {
	info := make([][]harfbuzz.GlyphInfo, 0, len(runs))
	pos := make([][]harfbuzz.GlyphPosition, 0, len(runs))
	for runIdx := range runs {
		run := &runs[runIdx]
		buf.Reset()
		var flags harfbuzz.BufferFlags
		if run.Start == 0 {
			flags |= harfbuzz.BufferFlagsBOT
		}
		if run.End == len(p.text.runes) {
			flags |= harfbuzz.BufferFlagsEOT
		}
		buf.SetFlags(flags)
		buf.SetClusterLevel(harfbuzz.ClusterLevelMonotoneCharacters)
		if lang, ok := run.runStyle.Language.Get(); ok {
			buf.SetLanguage(harfbuzz.LanguageFromString(lang.String()))
		}
		switch dir := run.Direction(); dir {
		case bidi.LeftToRight:
			buf.SetDirection(harfbuzz.LTR)
		case bidi.RightToLeft:
			buf.SetDirection(harfbuzz.RTL)
		default:
			panic(fmt.Sprintf("unhandled direction %v", dir))
		}
		buf.SetScript(opentype.Tag(run.script.String()))
		buf.AddRunes(p.text.runes, run.Start, run.End-run.Start)
		buf.GuessSegmentProperties()
		// XXX handle runs with no fonts

		// OPT it'd probably be better to create proper ranges of font features,
		// instead of setting them for each rune.
		var features []harfbuzz.Feature
		for runeIdx, runeStyle := range run.runeStyles {
			runeFeatures, _ := runeStyle.FontFeatures.Get()
			for _, ft := range runeFeatures {
				features = append(features, harfbuzz.Feature{
					Tag:   ft.Feature,
					Value: ft.Value,
					Start: runeIdx,
					End:   runeIdx + 1,
				})
			}
		}
		harfbuzz.Shape(run.font.hb, buf, features)

		if run.Direction() == bidi.RightToLeft {
			buf.ReverseClusters()
		}

		info = append(info, slices.Clone(buf.GlyphInfos()))
		pos = append(pos, slices.Clone(buf.GlyphPositions()))
	}

	return info, pos
}

func (p *Paragraph) runsCoveringInterval(start, end int) (int, int) {
	startRun, endRun := -1, -1
	for runIdx := range p.text.runs {
		run := &p.text.runs[runIdx]
		if run.Start <= start {
			startRun = runIdx
		}
		if run.Start >= end {
			endRun = runIdx
			break
		}
	}
	if endRun == -1 {
		endRun = len(p.text.runs)
	}
	if startRun == -1 || endRun == -1 {
		panic("unreachable")
	}
	return startRun, endRun
}

func (p *Paragraph) runsForInterval(start, end int) []run {
	startRun, endRun := p.runsCoveringInterval(start, end)
	if endRun-startRun == 0 {
		return nil
	}

	out := make([]run, 1, endRun-startRun)
	out[0] = p.text.runs[startRun]
	out[0].Start = start
	out[0].runeStyles = out[0].runeStyles[start-p.text.runs[startRun].Start:]

	if endRun-startRun == 1 {
		out[0].End = end
		return out
	}

	for i := startRun + 1; i < endRun-1; i++ {
		out = append(out, p.text.runs[i])
	}

	out = append(out, p.text.runs[endRun-1])
	out[len(out)-1].End = end

	return out
}

func (p *Paragraph) init() {
	ins := linebreak.Instance{}
	p.lb = ins.Process(p.text.runes)
	if len(p.lb.MandatoryBreaks) != 0 {
		// XXX figure out if we want Paragraph to be for a single paragraph or
		// not
		panic("unexpected mandatory line breaks")
	}

	buf := harfbuzz.NewBuffer()
	defer buf.Destroy()

	p.infos, p.poss = p.shapeRuns(buf, p.text.runs)
	extents := make([][]harfbuzz.GlyphExtents, len(p.infos))
	for i, infos := range p.infos {
		extents[i] = make([]harfbuzz.GlyphExtents, len(infos))
		for j, info := range infos {
			extents[i][j], _ = p.text.runs[i].font.hb.GlyphExtents(info.Codepoint)
		}
	}
	p.extents = extents

	for i := range p.text.runs {
		p.text.runs[i].glyphs = p.infos[i]
		p.text.runs[i].glyphPos = p.poss[i]
		p.text.runs[i].extents = p.extents[i]
	}
}

type bitset []uint64

func newBitset(n int) bitset {
	return make([]uint64, (n+63)/64)
}

func (bs bitset) get(idx int) bool {
	return (bs[idx/64]>>(idx%64))&1 != 0
}

func (bs bitset) set(idx int) {
	bs[idx/64] |= 1 << (idx % 64)
}

func (p *Paragraph) Layout(maxWidth float64) {
	// OPT don't do any line breaking if the entire paragraph fits on one line

	// safeToBreaks tracks for each rune whether breaking right before it is
	// safe.
	safeToBreaks := newBitset(len(p.text.runes) + 1)
	for _, run := range p.text.runs {
		for _, glyph := range run.glyphs {
			if glyph.Flags()&harfbuzz.GlyphFlagsUnsafeToBreak == 0 {
				safeToBreaks.set(int(glyph.Cluster))
			}
		}
	}
	safeToBreaks.set(len(p.text.runes))

	// safeWidths tracks for each rune the width of the text if we broke right
	// before the rune, assuming it is a safe break.
	safeWidths := make([]float64, len(p.text.runes)+1)
	origin := 0.0
	rightmost := 0.0
	for _, run := range p.text.runs {
		upem := run.font.hb.Face().UPEM()
		pxPerEm := run.runStyle.FontSize.UnwrapOr(0)
		scale := (2 * pxPerEm) / float64(upem)
		for i, info := range run.glyphs {
			extents := run.extents[i]
			pos := run.glyphPos[i]
			// FIXME this doesn't take the added width from stroking
			// into consideration

			// Only extent the width if we've actually put down any ink.
			// This allows lines to have trailing whitespace.
			rightmost = max(rightmost, origin+float64(pos.XOffset+extents.XBearing+extents.Width)*scale)
			safeWidths[info.Cluster+1] = rightmost
			origin += float64(pos.XAdvance) * scale
		}
	}

	// The start of the current line
	start := 0
	clear(p.lines)
	lines := p.lines[:0]
	for start < len(p.text.runes) {
		var curBestCandidate line
		for i := start + 1; i < len(p.lb.Breaks); i++ {
			if !p.lb.Breaks[i] {
				continue
			}

			if safeToBreaks.get(i) {
				width := safeWidths[i] - safeWidths[start]
				if width <= maxWidth {
					if width >= curBestCandidate.width {
						curBestCandidate = line{
							start: start,
							end:   i,
							width: width,
						}
					}
				}
			} else {
				panic("unimplemented")
			}
		}

		// XXX if all lines were too long, fall back to doing something.

		if curBestCandidate.runs == nil {
			curBestCandidate.runs = p.runsForInterval(curBestCandidate.start, curBestCandidate.end)
		}
		lines = append(lines, curBestCandidate)
		start = curBestCandidate.end
	}

	p.lines = lines
}

func (p *Paragraph) Paint(rec gfx.Recorder) {
	rec = rec.Checkpoint()

	// XXX figure out where this should get called from
	// XXX get actual available width
	const maxWidth = 601
	p.Layout(maxWidth)
	lines := p.lines

	y := 0.0
	for _, line := range lines {
		var maxAscender, maxDescender, maxGap float64
		for _, run := range line.runs {
			hz, ok := run.font.hb.HorizontalExtents()
			if !ok {
				// TODO what would we even do if the font doesn't have valid
				// metrics?
			}
			upem := run.font.hb.Face().UPEM()
			pxPerEm := run.runStyle.FontSize.UnwrapOr(0)
			scaleFactor := (2 * pxPerEm) / float64(upem)
			maxAscender = max(maxAscender, float64(hz.Ascender)*scaleFactor)
			maxDescender = min(maxDescender, float64(hz.Descender)*scaleFactor)
			maxGap = max(maxGap, float64(hz.LineGap)*scaleFactor)
		}

		y += maxAscender
		// Align baseline with logical pixel grid.
		y = math.Round(y)

		// XXX this isn't running L1 of the bidi algorithm, which is important for
		// trailing whitespace
		//
		// OPT don't do this work every time we paint. this essentially belongs
		// to Layout.
		runs := line.runs
		indices := make([]int, len(runs))

		// OPT if we used SoA we could directly pass the runs to ReorderRuns, but
		// AoS is simply more ergonomic. Maybe we can add a ReorderSeq and avoid
		// allocating the slice that way?
		bidiRuns := make([]bidi.Run, len(indices))
		for i := range runs {
			bidiRuns[i] = runs[i].Run
		}
		p.text.bidiParagraph.ReorderRuns(bidiRuns, indices)

		var origin curve.Point
		switch p.style.Direction {
		case bidi.LeftToRight:
			origin = curve.Pt(0, y)
		case bidi.RightToLeft:
			origin = curve.Pt(maxWidth, y)
		}

		do := func(runIdx int) {
			run := runs[runIdx]

			upem := run.font.hb.Face().UPEM()
			pxPerEm := run.runStyle.FontSize.UnwrapOr(0)
			scaleFactor := (2 * pxPerEm) / float64(upem)
			scale := curve.Scale(scaleFactor, scaleFactor)

			for i := range run.Glyphs(p.style.Direction) {
				glyph := run.glyphs[i]
				pos := run.glyphPos[i]
				extents := run.extents[i]

				oldOrigin := origin

				if p.style.Direction == bidi.RightToLeft {
					// We don't support vertical text, so no advance in the Y.
					origin = origin.Translate(curve.Vec2(curve.Pt(float64(-pos.XAdvance), 0).Transform(scale)))
				}

				glyphOffset := origin.Translate(
					curve.Vec(float64(pos.XOffset), -float64(pos.YOffset)).
						Mul(scaleFactor))

				// TODO(dh): support most of the fields from TextStyle

				style := run.runeStyles[int(glyph.Cluster)-run.Start]
				if fill, ok := style.Fill.Get(); ok {
					key := filledGlyphCacheKey{
						glyph: glyph.Codepoint,
						font:  run.font,
						color: fill,
					}
					glyphRec, ok := p.filledGlyphCache[key]
					if !ok {
						glyphScene := gfx.NewSimpleRecorder()
						run.font.PaintGlyph(fill, glyph.Codepoint, glyphScene)
						glyphRec = glyphScene.Finish()
						p.filledGlyphCache[key] = glyphRec
					}
					rec.PushTransform(scale.ThenTranslate(curve.Vec2(glyphOffset)))
					rec.PlayRecording(glyphRec)
					rec.PopTransform()
				}

				if stroke, ok := run.runeStyles[int(glyph.Cluster)-run.Start].Stroke.Get(); ok {
					path := run.font.GlyphOutline(glyph.Codepoint)
					style := stroke.Style
					style.Width /= scaleFactor
					style.MiterLimit /= scaleFactor
					style.DashOffset /= scaleFactor
					style.DashPattern = slices.Clone(style.DashPattern)
					for i := range style.DashPattern {
						style.DashPattern[i] /= scaleFactor
					}
					glyphScene := gfx.NewSimpleRecorder()
					glyphScene.PushTransform(scale.ThenTranslate(curve.Vec2(glyphOffset)))
					glyphScene.Stroke(
						path,
						style,
						gfx.Solid(stroke.Color),
					)
					glyphScene.PopTransform()
					rec.PlayRecording(glyphScene.Finish())
				}

				if debugText {
					rec.Fill(
						curve.Circle{
							Center: glyphOffset,
							Radius: 1.5,
						},
						gfx.Solid(color.Make(color.SRGB, 1, 0, 0, 1)),
					)

					bbox := curve.NewRectFromOrigin(
						curve.Point(curve.Vec(float64(extents.XBearing), float64(-extents.YBearing)).Mul(scaleFactor)),
						curve.Sz(float64(extents.Width), float64(-extents.Height)).Scale(scaleFactor),
					).Translate(curve.Vec2(glyphOffset))

					c := color.Make(color.SRGB, 1, 0, 1, 0.5)
					if glyph.Flags()&harfbuzz.GlyphFlagsUnsafeToBreak != 0 {
						c = color.Make(color.SRGB, 0, 0, 1, 0.5)
					}
					rec.Fill(
						bbox,
						gfx.Solid(c),
					)
				}

				// We don't support vertical text, so no advance in the Y.
				if p.style.Direction == bidi.LeftToRight {
					origin = origin.Translate(curve.Vec2(curve.Pt(float64(pos.XAdvance), 0).Transform(scale)))
				}

				if debugText {
					p0 := oldOrigin
					p1 := origin
					p1.Y += 5
					rec.Stroke(
						curve.NewRectFromPoints(curve.Point(p0), curve.Point(p1)),
						curve.DefaultStroke.WithJoin(curve.MiterJoin),
						gfx.Solid(color.Make(color.SRGB, 1, 0, 0, 1)),
					)
				}
			}
		}

		if p.style.Direction == bidi.RightToLeft {
			slices.Reverse(indices)
		}
		for _, idx := range indices {
			do(idx)
		}

		y += -maxDescender
		y += maxGap
	}
}

type LineMetrics struct {
	Ascent         float64
	Baseline       float64
	Descent        float64
	HardBreak      bool
	Height         float64
	Left           float64
	LineNumber     int
	UnscaledAscent float64
	Width          float64
}

type glyphPainter struct {
	// OPT a common pattern is 'push group; push clip; fill; pop clip; pop
	// group'. We could optimize the clip and fill to a simple fill with a path.
	// Often, the group's blend mode is the default, in which case we can
	// probably forego the layer as well.

	font *Font
	fg   color.Color

	layers     []gfx.Recorder
	transforms []curve.Affine
}

func (g *glyphPainter) Foreground() color.Color {
	return g.fg
}

// Fill implements harfbuzz.GlyphPainter.
func (g *glyphPainter) Fill(b gfx.Paint) {
	// XXX guard against empty layers
	g.layers[len(g.layers)-1].Fill(
		// FIXME use the glyph's clip box, if available
		curve.NewRectFromPoints(
			curve.Pt(-100_000, -100_000),
			curve.Pt(100_000, 100_000),
		),
		b,
	)
}

// ColorGlyph implements harfbuzz.GlyphPainter.
func (g *glyphPainter) ColorGlyph(glyph int32) bool {
	// XXX implement
	return false
}

// Image implements harfbuzz.GlyphPainter.
func (g *glyphPainter) Image(img image.Image, slant float64, extents harfbuzz.GlyphExtents) bool {
	// XXX implement
	return false
}

// PushClipGlyph implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PushClipGlyph(glyph int32) {
	path := g.font.GlyphOutline(glyph)
	// XXX guard against empty g.transforms
	// XXX get rid of g.transforms
	rec := g.layers[len(g.layers)-1]
	rec.PushTransform(g.transforms[len(g.transforms)-1])
	rec.PushClip(path)
	rec.PopTransform()
}

// PushClipRect implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PushClipRect(rect curve.Rect) {
	// XXX guard against empty g.transforms
	// XXX get rid of g.transforms
	rec := g.layers[len(g.layers)-1]
	rec.PushTransform(g.transforms[len(g.transforms)-1])
	rec.PushClip(rect)
	rec.PopTransform()
}

// PopClip implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PopClip() {
	// XXX guard against empty layers
	g.layers[len(g.layers)-1].PopLayer()
}

// PushGroup implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PushGroup() {
	g.layers = append(g.layers, gfx.NewSimpleRecorder())
}

// PopGroup implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PopGroup(mode gfx.BlendMode) {
	if len(g.layers) == 0 {
		return
	}
	if len(g.layers) < 2 {
		g.layers = g.layers[:len(g.layers)-1]
		return
	}
	parent := g.layers[len(g.layers)-2]
	top := g.layers[len(g.layers)-1]
	g.layers = g.layers[:len(g.layers)-1]
	parent.PushLayer(
		gfx.Layer{
			BlendMode: mode,
			Opacity:   1,
		},
	)
	// XXX do proper transform handling
	parent.PushTransform(g.transforms[len(g.transforms)-1])
	parent.PlayRecording(top.Finish())
	parent.PopTransform()
	parent.PopLayer()
}

// PushTransform implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PushTransform(t curve.Affine) {
	if len(g.transforms) == 0 {
		g.transforms = append(g.transforms, t)
	} else {
		g.transforms = append(g.transforms, t.Mul(g.transforms[len(g.transforms)-1]))
	}
}

// PopTransform implements harfbuzz.GlyphPainter.
func (g *glyphPainter) PopTransform() {
	if len(g.transforms) == 0 {
		return
	}
	g.transforms = g.transforms[:len(g.transforms)-1]
}

type glyphDrawer struct {
	path curve.BezPath
}

// ClosePath implements harfbuzz.GlyphDrawer.
func (g *glyphDrawer) ClosePath() {
	g.path.ClosePath()
}

// CubicTo implements harfbuzz.GlyphDrawer.
func (g *glyphDrawer) CubicTo(cx1, cy1, cx2, cy2, x, y float64,
) {
	g.path.CubicTo(curve.Pt(cx1, -cy1), curve.Pt(cx2, -cy2), curve.Pt(x, -y))
}

// LineTo implements harfbuzz.GlyphDrawer.
func (g *glyphDrawer) LineTo(x, y float64) {
	g.path.LineTo(curve.Pt(x, -y))
}

// MoveTo implements harfbuzz.GlyphDrawer.
func (g *glyphDrawer) MoveTo(x, y float64) {
	g.path.MoveTo(curve.Pt(x, -y))
}

type Font struct {
	hb *harfbuzz.Font
	// TODO do we want to make fonts safe for concurrent use?
	glyphCache *tinylfu.T[int32, curve.BezPath]
}

func (f *Font) PaintGlyph(fg color.Color, gid int32, rec gfx.Recorder) {
	gp := &glyphPainter{
		font:       f,
		fg:         fg,
		transforms: []curve.Affine{curve.Identity},
		layers:     []gfx.Recorder{gfx.NewSimpleRecorder()},
	}
	f.hb.PaintGlyph(gid, gp)
	// XXX guard against mismatched group/pop group
	rec.PlayRecording(gp.layers[0].Finish())
}

func (f *Font) GlyphOutline(gid int32) curve.BezPath {
	if p, ok := f.glyphCache.Get(gid); ok {
		return p
	}
	d := &glyphDrawer{}
	f.hb.DrawGlyph(gid, d)
	f.glyphCache.Add(gid, d.path)
	return d.path
}

type ParagraphBuilder struct {
	style            *ParagraphStyle
	currentRunStyle  runStyle
	currentRuneStyle *Style
	textStyleStack   []*Style

	fonts map[fontKey]*Font

	text text
}

type fontKey struct {
	face       Face
	axisValues string
}

func makeFontKey(face Face, axisValues map[opentype.Tag]float64) fontKey {
	keys := slices.Collect(maps.Keys(axisValues))
	slices.Sort(keys)

	var s strings.Builder
	for _, k := range keys {
		v := axisValues[k]
		s.WriteString(string(k))
		var vb [8]byte
		binary.LittleEndian.PutUint64(vb[:], math.Float64bits(v))
		s.WriteString(string(vb[:]))
	}

	return fontKey{
		face:       face,
		axisValues: s.String(),
	}
}

func NewParagraphBuilder(pstyle *ParagraphStyle) *ParagraphBuilder {
	s := MakeDefaultStyle()
	rs := s.runStyle()
	return &ParagraphBuilder{
		style:            pstyle,
		currentRunStyle:  rs,
		currentRuneStyle: s,
		text: text{
			runs: []run{{runStyle: rs}},
		},
		fonts: make(map[fontKey]*Font),
	}
}

func (s *Style) runStyle() runStyle {
	return runStyle{
		FontFamilies: s.FontFamilies,
		FontSize:     s.FontSize,
		FontWeight:   s.FontWeight,
		FontWidth:    s.FontWidth,
		FontStyle:    s.FontStyle,
		Language:     s.Language,
	}
}

func (pb *ParagraphBuilder) PushStyle(ts *Style) *Style {
	ms := pb.currentRuneStyle.Merge(ts)
	pb.textStyleStack = append(pb.textStyleStack, pb.currentRuneStyle)
	pb.currentRuneStyle = ms
	pb.currentRunStyle = ms.runStyle()
	pb.startStyleRun()
	return pb.currentRuneStyle
}

func (pb *ParagraphBuilder) startStyleRun() {
	last := &pb.text.runs[len(pb.text.runs)-1]
	if last.End <= last.Start {
		// The current run is empty, so just change its style
		pb.text.runs[len(pb.text.runs)-1].runStyle = pb.currentRunStyle
		if len(pb.text.runs) > 1 &&
			pb.text.runs[len(pb.text.runs)-2].runStyle.Equal(&pb.currentRunStyle) {
			// Merge two runs with identical style
			pb.text.runs = pb.text.runs[:len(pb.text.runs)-1]
		}
		return
	}

	if pb.currentRunStyle.Equal(&last.runStyle) {
		return
	}

	pb.text.runs = append(pb.text.runs, run{
		Run: bidi.Run{
			Start: last.End,
			End:   last.End,
		},
		runStyle: pb.currentRunStyle,
	})
}

func (pb *ParagraphBuilder) PopStyle() {
	if len(pb.textStyleStack) == 0 {
		panic("ParagraphBuilder.PopStyle called with empty stack")
	}
	style := pb.textStyleStack[len(pb.textStyleStack)-1]
	pb.currentRuneStyle = style
	pb.currentRunStyle = style.runStyle()
	pb.textStyleStack = pb.textStyleStack[:len(pb.textStyleStack)-1]
	pb.startStyleRun()
}

func (pb *ParagraphBuilder) AddString(s string) {
	n := utf8.RuneCountInString(s)
	pb.text.runes = slices.Grow(pb.text.runes, n)
	for _, r := range s {
		pb.text.runes = append(pb.text.runes, r)
	}
	run := &pb.text.runs[len(pb.text.runs)-1]
	run.runeStyles = slices.Grow(run.runeStyles, n)
	for range n {
		run.runeStyles = append(run.runeStyles, pb.currentRuneStyle)
	}
	run.End += n
}

type run struct {
	bidi.Run
	runStyle runStyle
	script   xlanguage.Script
	font     *Font

	// OPT instead of storing one pointer per rune, store 32 bit indices to half
	// memory usage for this slice.
	runeStyles []*Style

	glyphs   []harfbuzz.GlyphInfo
	glyphPos []harfbuzz.GlyphPosition
	extents  []harfbuzz.GlyphExtents
}

func (r *run) Glyphs(mainDir bidi.Direction) iter.Seq[int] {
	if r.Direction() == mainDir {
		return func(yield func(int) bool) {
			for i := range r.glyphs {
				// OPT consider using binary search to find the first glyph. or
				// maybe we can just populate run.glyphs correctly in the first
				// place.
				if int(r.glyphs[i].Cluster) < r.Start {
					continue
				}
				if int(r.glyphs[i].Cluster) >= r.End {
					break
				}
				if !yield(i) {
					break
				}
			}
		}
	} else {
		return func(yield func(int) bool) {
			for i := len(r.glyphs) - 1; i >= 0; i-- {
				if int(r.glyphs[i].Cluster) >= r.End {
					continue
				}
				if int(r.glyphs[i].Cluster) < r.Start {
					break
				}
				if !yield(i) {
					break
				}
			}
		}
	}
}

type text struct {
	runes         []rune
	bidiParagraph bidi.Paragraph

	runs []run
}

func (pb *ParagraphBuilder) Build(fdb *fontdb.Faces, fl *FontLoader) *Paragraph {
	// Remove trailing empty run
	if len(pb.text.runs) > 0 {
		if last := &pb.text.runs[len(pb.text.runs)-1]; last.Start == last.End {
			pb.text.runs = pb.text.runs[:len(pb.text.runs)-1]
		}
	}

	pb.splitByBidi()
	pb.splitByScript()
	pb.splitByFont(fdb, fl)

	up := &Paragraph{
		style:            pb.style,
		text:             pb.text,
		filledGlyphCache: make(map[filledGlyphCacheKey]gfx.Recording),
	}
	up.init()
	return up
}
