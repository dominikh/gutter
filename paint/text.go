// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package paint

// FIXME(dh): we split runs based on style attributes like text color. will
// harfbuzz, using context, kern or form ligatures across these runs? if so,
// line wrapping may incorrectly split between two runs without knowing that it
// has to reshape.

// XXX do we have to segment text runs based on bidi overrides, too? (U+202D) that's probably part
// of the general segmenting by script and bidi.

// XXX segment text spans

// XXX optical bounds / https://github.com/harfbuzz/harfbuzz/issues/3458 / https://typedrawers.com/discussion/4527/glyph-sidebearings-and-text-alignment / opbd

import (
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/fontdb"
	"honnef.co/go/gutter/text"
	"honnef.co/go/gutter/text/bidi"
)

// Selecting fonts: we map BCP 47 tags to lists of fonts. Each font is tried in order until one covers the requested
// rune.
//
// If none of the available fonts cover a rune we have one of two options: 1) scan system
// fonts for one that covers the rune 2) fail. This choice can be made by including the
// font group {"system"}, which falls back to scanning system fonts. Not using system
// fonts is especially useful when bundling fonts and wanting to fail instead of falling
// back to arbitrary fonts for missing glyphs.
//
// BCP tags might omit any of the three elements. for example, und-Latn suffices as all
// uses of the latin script can use the same font, but we may have ru-Cyrl and bg-Cyrl, as
// Russia and Bulgaria use different variations of the same characters.
//
// When there is no configured or available font for a tag, we try less precise tags,
// first dropping region and then language.
//
// We don't rely on the OS for our defaults, because most systems don't provide reliable
// ways of determining per language/script defaults, or aren't properly configured to do
// so.
//
// The tags used for lookups will always include the script as determined by script
// segmentation, even if it is a suppressed script. Script segmentation is based on
// Unicode, which doesn't differentiate between Simplified and Traditional Chinese. As
// such, entries in FontFamilies shouldn't use Hant or Hans but only Hani.

// Note: serif and sans-serif only apply to a small handful of writing scripts. Their use as generic font families in
// CSS is historical, and reflects the Latin-centric nature of early Web development. Better and more widely applicable
// names would have been, for example, "modulated" and "monoline". However, for reasons of Web compatibility, these
// names cannot be changed.
//
// CSS uses the term "serif" to apply to a font for any script, although other names might be more familiar for
// particular scripts, such as Mincho (Japanese), Sung or Song (Chinese), Batang (Korean). For Arabic, the Naskh style
// would correspond to serif.
//
// 	CSS uses the term "sans-serif" to apply to a font for any script, although other names might be more familiar for
// 	 particular scripts, such as Gothic (Japanese), Hei (Chinese), or Gulim (Korean).

// generic(fangsong) This font family is used for Fang Song (仿宋) typefaces in Chinese. Fang Song is a relaxed,
//     intermediate form between Song (serif) and Kai (generic(kai)). Typically, the horizontal lines are tilted, the
//     endpoint flourishes are smaller, and there is less variation in stroke width, compared to a Song style. Fang Song
//     is often used for official Chinese Government documents.

//     Note: generic(fangsong) might not map to any locally installed font, but if it does, that font will be in Fang
//     Song style.

// generic(kai) This font family is used in Simplified & Traditional Chinese. A major typeface, which provides
//     calligraphic styles for Chinese text. It shows notable handwriting features. Kai is commonly used in official
//     documents and textbooks. Most official documents in Taiwan use Kai in full text. Kai can also be combined with
//     other typefaces to be used in text that needs to be differentiated from the rest of the content, for example,
//     headlines, references, quotations, and dialogs.

//     Note: generic(kai) might not map to any locally installed font, but if it does, that font will be in Kai style.

// generic(nastaliq) This font family is the standard way of writing Urdu and Kashmiri, and is also often a preferred
//     style for Persian and other language text, especially in literary genres such as poetry. Key features include a
//     sloping baseline for joined letters, and overall complex shaping and positioning for base letters and diacritics
//     alike. There are also distinctive shapes for many glyphs and ligatures. It is important not to fall back to a
//     naskh style for languages such as Urdu and Kashmiri.

//     Note: generic(nastaliq) might not map to any locally installed font, but if it does, that font will be in
//     Nastaliq style.

// XXX do we need an entry for Kashmiri?

// Font name aliases.
var Aliases = map[string]string{
	"ヒラギノ角ゴ ProN": "Hiragino Kaku Gothic ProN",
	"游ゴシック":       "Yu Gothic",
	"游ゴシック体":      "Yu Gothic",
	"メイリオ":        "Meiryo",
	"ヒラギノ明朝 ProN": "Hiragino Mincho ProN",
	"游明朝":         "Yu Mincho",
	"游明朝体":        "Yu Mincho",

	// We don't alias the various Noto Sans CJK fonts, because if the user explicitly
	// specifies a language variant, they expect to get the font that defaults to that
	// language.
}

type PlaceholderAlignment int

const (
	PlaceholderAlignmentBaseline PlaceholderAlignment = iota
	PlaceholderAlignmentAboveBaseline
	PlaceholderAlignmentBelowBaseline
	PlaceholderAlignmentTop
	PlaceholderAlignmentBottom
	PlaceholderAlignmentMiddle
)

type TextBaseline int

const (
	TextBaselineAlphabetic TextBaseline = iota
	TextBaselineIdeographic
)

type PlaceholderDimensions struct {
	Size           curve.Size
	Alignment      PlaceholderAlignment
	Baseline       TextBaseline
	BaselineOffset float64
}

type TextBox struct {
	Rect      curve.Rect
	Direction bidi.Direction
}

func (tb *TextBox) Start() float64 {
	switch tb.Direction {
	case bidi.LeftToRight:
		return tb.Rect.X0
	case bidi.RightToLeft:
		return tb.Rect.X1
	default:
		return 0
	}
}

func (tb *TextBox) End() float64 {
	switch tb.Direction {
	case bidi.LeftToRight:
		return tb.Rect.X1
	case bidi.RightToLeft:
		return tb.Rect.X0
	default:
		return 0
	}
}

type InlineSpan interface {
	Build(pb *text.ParagraphBuilder, dimensions []PlaceholderDimensions)
}

type PlaceholderSpan interface {
	InlineSpan
}

type TextSpan struct {
	Text     string
	Children []InlineSpan
	Style    text.Style
	// XXX gesture recognizer
	// XXX mouse cursor
	// XXX pointer event and exit event listener
}

func (txt *TextSpan) Build(pb *text.ParagraphBuilder, dimensions []PlaceholderDimensions) {
	pb.PushStyle(&txt.Style)
	defer pb.PopStyle()
	pb.AddString(txt.Text)

	for _, child := range txt.Children {
		child.Build(pb, dimensions)
	}
}

type TextPainter struct {
	text          InlineSpan
	textAlignment text.Alignment
	textDirection bidi.Direction
	// XXX textScaler
	maxLines int
	ellipsis string
	// XXX strutStyle
	// XXX textWidthBasis
	textHeightBehavior    text.HeightBehavior
	placeholderDimensions []PlaceholderDimensions

	layoutCache *textPainterLayoutCacheWithOffset
}

type textPainterLayoutCacheWithOffset struct{}

func (tp *TextPainter) Text() InlineSpan { return tp.text }
func (tp *TextPainter) SetText(text InlineSpan) {
	if tp.text == text {
		return
	}
	tp.text = text
	// OPT(dh): some text changes may only need a paint change, or no change at all, in which case we can
	// avoid computing the layout. In Flutter, this is done via InlineSpan.compareTo.
	tp.markNeedsLayout()
}

func (tp *TextPainter) TextAlignment() text.Alignment { return tp.textAlignment }
func (tp *TextPainter) SetTextAlignment(a text.Alignment) {
	if tp.textAlignment == a {
		return
	}
	tp.textAlignment = a
	tp.markNeedsLayout()
}

func (tp *TextPainter) TextDirection() bidi.Direction { return tp.textDirection }
func (tp *TextPainter) SetTextDirection(d bidi.Direction) {
	if tp.textDirection == d {
		return
	}
	tp.textDirection = d
	tp.markNeedsLayout()
}

func (tp *TextPainter) Ellipsis() string { return tp.ellipsis }
func (tp *TextPainter) SetEllipsis(s string) {
	if tp.ellipsis == s {
		return
	}
	tp.ellipsis = s
	tp.markNeedsLayout()
}

func (tp *TextPainter) MaxLines() int { return tp.maxLines }
func (tp *TextPainter) SetMaxLines(n int) {
	if tp.maxLines == n {
		return
	}
	tp.maxLines = n
	tp.markNeedsLayout()
}

func (tp *TextPainter) TextHeightBehavior() text.HeightBehavior { return tp.textHeightBehavior }
func (tp *TextPainter) SetTextHeightBehavior(hb text.HeightBehavior) {
	if tp.textHeightBehavior == hb {
		return
	}
	tp.textHeightBehavior = hb
	tp.markNeedsLayout()
}

func (tp *TextPainter) markNeedsLayout() {
	tp.layoutCache = nil
}

func (tp *TextPainter) Layout(minWidth, maxWidth float64) *text.Paragraph {
	// XXX what is minWidth even for

	debug.Assert(!math.IsNaN(float64(minWidth)))
	debug.Assert(!math.IsNaN(float64(maxWidth)))

	ps := text.ParagraphStyle{
		Alignment:          tp.textAlignment,
		Direction:          tp.textDirection,
		MaxLines:           tp.maxLines,
		Ellipsis:           tp.ellipsis,
		TextHeightBehavior: tp.textHeightBehavior,
	}

	pb := text.NewParagraphBuilder(&ps)
	tp.text.Build(pb, nil)
	// XXX fdb and fl should be reused
	fdb := fontdb.New()
	fl := new(text.FontLoader)
	p := pb.Build(fdb, fl)
	p.Layout(maxWidth)

	return p

	// OPT(dh): changes to the width do not affect layout/line-wrapping if both the old
	// and new width exceed the text's intrinsic width, and in that case we can skip doing
	// layout.

	// TODO(dh): what's the meaning of an infinite maxWidth with text that isn't
	// left-aligned? Flutter seems to use the computed maxIntrinsicLineExtent as the width
	// in that case.

	// OPT(dh): don't eagerly layout if we're only laying out to update paint information.
	// the user might never call paint (e.g. when only taking measurements), or might
	// change paint attributes again.
}

func (tp *TextPainter) PlaceholderDimensions() []PlaceholderDimensions {
	return tp.placeholderDimensions
}

func (tp *TextPainter) SetPlaceholderDimensions(dims []PlaceholderDimensions) {
	tp.placeholderDimensions = dims
	tp.markNeedsLayout()
}
