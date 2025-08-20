// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"honnef.co/go/curve"
	"honnef.co/go/stuff/container/maybe"
	"honnef.co/go/gutter/paint"
	"honnef.co/go/gutter/text"
	"honnef.co/go/gutter/text/bidi"
)

var _ Object = (*Paragraph)(nil)

type ParagraphAttributes struct {
	Text          paint.InlineSpan
	TextAlign     text.Alignment
	TextDirection bidi.Direction
	SingleLine    bool
	Overflow      text.Overflow
	// XXX textScaler
	MaxLines maybe.Option[int]
	// Language maybe.Option[paint.Language]
	// XXX StrutStyle
	// XXX TextWidthBasis
	// XXX textHeightBehavior (why is there no height field?)
	// XXX selection registrar
	// XXX selection color
}

// XXX move into the render package and rename
type Paragraph struct {
	ManyChildren

	attrs ParagraphAttributes
}

func NewParagraph(attrs ParagraphAttributes) *Paragraph {
	return &Paragraph{
		attrs: attrs,
	}
}

// XXX add setters for Paragraph fields

// Handle implements Object.
func (r *Paragraph) Handle() *ObjectHandle {
	panic("unimplemented")
}

// PerformLayout implements Object.
func (r *Paragraph) PerformLayout() (size curve.Size) {
	panic("unimplemented")
}

// PerformPaint implements Object.
func (*Paragraph) PerformPaint(p *Painter) {
	panic("unimplemented")
}

// VisitChildren implements Object.
func (r *Paragraph) VisitChildren(yield func(Object) bool) {
	panic("unimplemented")
}

/*
/// A render object that displays a paragraph of text.
class RenderParagraph extends RenderBox with ContainerRenderObjectMixin<RenderBox, TextParentData>, RenderInlineChildrenContainerDefaults {
  /// Creates a paragraph render object.
  ///
  /// The [maxLines] property may be null (and indeed defaults to null), but if
  /// it is not null, it must be greater than zero.
  RenderParagraph(InlineSpan text, {
    TextAlign textAlign = TextAlign.start,
    required TextDirection textDirection,
    bool softWrap = true,
    TextOverflow overflow = TextOverflow.clip,
    @Deprecated(
      'Use textScaler instead. '
      'Use of textScaleFactor was deprecated in preparation for the upcoming nonlinear text scaling support. '
      'This feature was deprecated after v3.12.0-2.0.pre.',
    )
    double textScaleFactor = 1.0,
    TextScaler textScaler = TextScaler.noScaling,
    int? maxLines,
    Locale? locale,
    StrutStyle? strutStyle,
    TextWidthBasis textWidthBasis = TextWidthBasis.parent,
    ui.TextHeightBehavior? textHeightBehavior,
    List<RenderBox>? children,
  }) : assert(text.debugAssertIsValid()),
       assert(maxLines == null || maxLines > 0),
       assert(
         identical(textScaler, TextScaler.noScaling) || textScaleFactor == 1.0,
         'textScaleFactor is deprecated and cannot be specified when textScaler is specified.',
       ),
       _softWrap = softWrap,
       _overflow = overflow,
       _textPainter = TextPainter(
         text: text,
         textAlign: textAlign,
         textDirection: textDirection,
         textScaler: textScaler == TextScaler.noScaling ? TextScaler.linear(textScaleFactor) : textScaler,
         maxLines: maxLines,
         ellipsis: overflow == TextOverflow.ellipsis ? _kEllipsis : null,
         locale: locale,
         strutStyle: strutStyle,
         textWidthBasis: textWidthBasis,
         textHeightBehavior: textHeightBehavior,
       ) {
    addAll(children);
    this.registrar = registrar;
  }

  static final String _placeholderCharacter = String.fromCharCode(PlaceholderSpan.placeholderCodeUnit);

  final TextPainter _textPainter;

  List<AttributedString>? _cachedAttributedLabels;


  /// The text to display.
  InlineSpan get text => _textPainter.text!;
  set text(InlineSpan value) {
    switch (_textPainter.text!.compareTo(value)) {
      case RenderComparison.identical:
        return;
      case RenderComparison.metadata:
        _textPainter.text = value;
      case RenderComparison.paint:
        _textPainter.text = value;
        _cachedAttributedLabels = null;
        markNeedsPaint();
      case RenderComparison.layout:
        _textPainter.text = value;
        _cachedAttributedLabels = null;
        markNeedsLayout();
    }
  }

  /// How the text should be aligned horizontally.
  TextAlign get textAlign => _textPainter.textAlign;
  set textAlign(TextAlign value) {
    if (_textPainter.textAlign == value) {
      return;
    }
    _textPainter.textAlign = value;
    markNeedsPaint();
  }

  /// The directionality of the text.
  ///
  /// This decides how the [TextAlign.start], [TextAlign.end], and
  /// [TextAlign.justify] values of [textAlign] are interpreted.
  ///
  /// This is also used to disambiguate how to render bidirectional text. For
  /// example, if the [text] is an English phrase followed by a Hebrew phrase,
  /// in a [TextDirection.ltr] context the English phrase will be on the left
  /// and the Hebrew phrase to its right, while in a [TextDirection.rtl]
  /// context, the English phrase will be on the right and the Hebrew phrase on
  /// its left.
  TextDirection get textDirection => _textPainter.textDirection!;
  set textDirection(TextDirection value) {
    if (_textPainter.textDirection == value) {
      return;
    }
    _textPainter.textDirection = value;
    markNeedsLayout();
  }

  /// Whether the text should break at soft line breaks.
  ///
  /// If false, the glyphs in the text will be positioned as if there was
  /// unlimited horizontal space.
  ///
  /// If [softWrap] is false, [overflow] and [textAlign] may have unexpected
  /// effects.
  bool get softWrap => _softWrap;
  bool _softWrap;
  set softWrap(bool value) {
    if (_softWrap == value) {
      return;
    }
    _softWrap = value;
    markNeedsLayout();
  }

  /// How visual overflow should be handled.
  TextOverflow get overflow => _overflow;
  TextOverflow _overflow;
  set overflow(TextOverflow value) {
    if (_overflow == value) {
      return;
    }
    _overflow = value;
    _textPainter.ellipsis = value == TextOverflow.ellipsis ? _kEllipsis : null;
    markNeedsLayout();
  }

  /// Deprecated. Will be removed in a future version of Flutter. Use
  /// [textScaler] instead.
  ///
  /// The number of font pixels for each logical pixel.
  ///
  /// For example, if the text scale factor is 1.5, text will be 50% larger than
  /// the specified font size.
  @Deprecated(
    'Use textScaler instead. '
    'Use of textScaleFactor was deprecated in preparation for the upcoming nonlinear text scaling support. '
    'This feature was deprecated after v3.12.0-2.0.pre.',
  )
  double get textScaleFactor => _textPainter.textScaleFactor;
  @Deprecated(
    'Use textScaler instead. '
    'Use of textScaleFactor was deprecated in preparation for the upcoming nonlinear text scaling support. '
    'This feature was deprecated after v3.12.0-2.0.pre.',
  )
  set textScaleFactor(double value) {
    textScaler = TextScaler.linear(value);
  }

  /// {@macro flutter.painting.textPainter.textScaler}
  TextScaler get textScaler => _textPainter.textScaler;
  set textScaler(TextScaler value) {
    if (_textPainter.textScaler == value) {
      return;
    }
    _textPainter.textScaler = value;
    markNeedsLayout();
  }

  /// An optional maximum number of lines for the text to span, wrapping if
  /// necessary. If the text exceeds the given number of lines, it will be
  /// truncated according to [overflow] and [softWrap].
  int? get maxLines => _textPainter.maxLines;
  /// The value may be null. If it is not null, then it must be greater than
  /// zero.
  set maxLines(int? value) {
    assert(value == null || value > 0);
    if (_textPainter.maxLines == value) {
      return;
    }
    _textPainter.maxLines = value;
    markNeedsLayout();
  }

  /// Used by this paragraph's internal [TextPainter] to select a
  /// locale-specific font.
  ///
  /// In some cases, the same Unicode character may be rendered differently
  /// depending on the locale. For example, the '骨' character is rendered
  /// differently in the Chinese and Japanese locales. In these cases, the
  /// [locale] may be used to select a locale-specific font.
  Locale? get locale => _textPainter.locale;
  /// The value may be null.
  set locale(Locale? value) {
    if (_textPainter.locale == value) {
      return;
    }
    _textPainter.locale = value;
    markNeedsLayout();
  }

  /// {@macro flutter.painting.textPainter.strutStyle}
  StrutStyle? get strutStyle => _textPainter.strutStyle;
  /// The value may be null.
  set strutStyle(StrutStyle? value) {
    if (_textPainter.strutStyle == value) {
      return;
    }
    _textPainter.strutStyle = value;
    markNeedsLayout();
  }

  /// {@macro flutter.painting.textPainter.textWidthBasis}
  TextWidthBasis get textWidthBasis => _textPainter.textWidthBasis;
  set textWidthBasis(TextWidthBasis value) {
    if (_textPainter.textWidthBasis == value) {
      return;
    }
    _textPainter.textWidthBasis = value;
    markNeedsLayout();
  }

  /// {@macro dart.ui.textHeightBehavior}
  ui.TextHeightBehavior? get textHeightBehavior => _textPainter.textHeightBehavior;
  set textHeightBehavior(ui.TextHeightBehavior? value) {
    if (_textPainter.textHeightBehavior == value) {
      return;
    }
    _textPainter.textHeightBehavior = value;
    markNeedsLayout();
  }

  Offset _getOffsetForPosition(TextPosition position) {
    return getOffsetForCaret(position, Rect.zero) + Offset(0, getFullHeightForCaret(position) ?? 0.0);
  }

  List<ui.LineMetrics> _computeLineMetrics() {
    return _textPainter.computeLineMetrics();
  }


  @override
  double computeDistanceToActualBaseline(TextBaseline baseline) {
    assert(!debugNeedsLayout);
    assert(constraints.debugAssertIsValid());
    _layoutTextWithConstraints(constraints);
    // TODO(garyq): Since our metric for ideographic baseline is currently
    // inaccurate and the non-alphabetic baselines are based off of the
    // alphabetic baseline, we use the alphabetic for now to produce correct
    // layouts. We should eventually change this back to pass the `baseline`
    // property when the ideographic baseline is properly implemented
    // (https://github.com/flutter/flutter/issues/22625).
    return _textPainter.computeDistanceToActualBaseline(TextBaseline.alphabetic);
  }

  /// Whether all inline widget children of this [RenderBox] support dry layout
  /// calculation.
  bool _canComputeDryLayoutForInlineWidgets() {
    // Dry layout cannot be calculated without a full layout for
    // alignments that require the baseline (baseline, aboveBaseline,
    // belowBaseline).
    return text.visitChildren((InlineSpan span) {
      return (span is! PlaceholderSpan) || switch (span.alignment) {
        ui.PlaceholderAlignment.baseline ||
        ui.PlaceholderAlignment.aboveBaseline ||
        ui.PlaceholderAlignment.belowBaseline => false,
        ui.PlaceholderAlignment.top ||
        ui.PlaceholderAlignment.middle ||
        ui.PlaceholderAlignment.bottom => true,
      };
    });
  }


  @override
  bool hitTestSelf(Offset position) => true;

  @override
  @protected
  bool hitTestChildren(BoxHitTestResult result, { required Offset position }) {
    final GlyphInfo? glyph = _textPainter.getClosestGlyphForOffset(position);
    // The hit-test can't fall through the horizontal gaps between visually
    // adjacent characters on the same line, even with a large letter-spacing or
    // text justification, as graphemeClusterLayoutBounds.width is the advance
    // width to the next character, so there's no gap between their
    // graphemeClusterLayoutBounds rects.
    final InlineSpan? spanHit = glyph != null && glyph.graphemeClusterLayoutBounds.contains(position)
      ? _textPainter.text!.getSpanForPosition(TextPosition(offset: glyph.graphemeClusterCodeUnitRange.start))
      : null;
    switch (spanHit) {
      case final HitTestTarget span:
        result.add(HitTestEntry(span));
        return true;
      case _:
        return hitTestInlineChildren(result, position);
    }
  }

  bool _needsClipping = false;

  void _layoutText({ double minWidth = 0.0, double maxWidth = double.infinity }) {
    final bool widthMatters = softWrap || overflow == TextOverflow.ellipsis;
    _textPainter.layout(
      minWidth: minWidth,
      maxWidth: widthMatters ? maxWidth : double.infinity,
    );
  }

  void _layoutTextWithConstraints(BoxConstraints constraints) {
    _textPainter.setPlaceholderDimensions(_placeholderDimensions);
    _layoutText(minWidth: constraints.minWidth, maxWidth: constraints.maxWidth);
  }

  @override
  void performLayout() {
    final BoxConstraints constraints = this.constraints;
    _placeholderDimensions = layoutInlineChildren(constraints.maxWidth, ChildLayoutHelper.layoutChild);
    _layoutTextWithConstraints(constraints);
    positionInlineChildren(_textPainter.inlinePlaceholderBoxes!);

    // We grab _textPainter.size and _textPainter.didExceedMaxLines here because
    // assigning to `size` will trigger us to validate our intrinsic sizes,
    // which will change _textPainter's layout because the intrinsic size
    // calculations are destructive. Other _textPainter state will also be
    // affected. See also RenderEditable which has a similar issue.
    final Size textSize = _textPainter.size;
    final bool textDidExceedMaxLines = _textPainter.didExceedMaxLines;
    size = constraints.constrain(textSize);

    final bool didOverflowHeight = size.height < textSize.height || textDidExceedMaxLines;
    final bool didOverflowWidth = size.width < textSize.width;
    // TODO(abarth): We're only measuring the sizes of the line boxes here. If
    // the glyphs draw outside the line boxes, we might think that there isn't
    // visual overflow when there actually is visual overflow. This can become
    // a problem if we start having horizontal overflow and introduce a clip
    // that affects the actual (but undetected) vertical overflow.
    final bool hasVisualOverflow = didOverflowWidth || didOverflowHeight;
    if (hasVisualOverflow) {
      switch (_overflow) {
        case TextOverflow.visible:
          _needsClipping = false;
        case TextOverflow.clip:
        case TextOverflow.ellipsis:
          _needsClipping = true;
      }
    } else {
      _needsClipping = false;
    }
  }

  @override
  void applyPaintTransform(RenderBox child, Matrix4 transform) {
    defaultApplyPaintTransform(child, transform);
  }

  @override
  void paint(PaintingContext context, Offset offset) {
    // Ideally we could compute the min/max intrinsic width/height with a
    // non-destructive operation. However, currently, computing these values
    // will destroy state inside the painter. If that happens, we need to get
    // back the correct state by calling _layout again.
    //
    // TODO(abarth): Make computing the min/max intrinsic width/height a
    //  non-destructive operation.
    //
    // If you remove this call, make sure that changing the textAlign still
    // works properly.
    _layoutTextWithConstraints(constraints);

    if (_needsClipping) {
      final Rect bounds = offset & size;
      context.canvas.save();
      context.canvas.clipRect(bounds);
    }

    _textPainter.paint(context.canvas, offset);

    paintInlineChildren(context, offset);

    if (_needsClipping) {
      context.canvas.restore();
    }
  }

  /// Returns the offset at which to paint the caret.
  ///
  /// Valid only after [layout].
  Offset getOffsetForCaret(TextPosition position, Rect caretPrototype) {
    assert(!debugNeedsLayout);
    _layoutTextWithConstraints(constraints);
    return _textPainter.getOffsetForCaret(position, caretPrototype);
  }

  /// {@macro flutter.painting.textPainter.getFullHeightForCaret}
  ///
  /// Valid only after [layout].
  double? getFullHeightForCaret(TextPosition position) {
    assert(!debugNeedsLayout);
    _layoutTextWithConstraints(constraints);
    return _textPainter.getFullHeightForCaret(position, Rect.zero);
  }

  /// Returns the position within the text for the given pixel offset.
  ///
  /// Valid only after [layout].
  TextPosition getPositionForOffset(Offset offset) {
    assert(!debugNeedsLayout);
    _layoutTextWithConstraints(constraints);
    return _textPainter.getPositionForOffset(offset);
  }

  /// Returns the text range of the word at the given offset. Characters not
  /// part of a word, such as spaces, symbols, and punctuation, have word breaks
  /// on both sides. In such cases, this method will return a text range that
  /// contains the given text position.
  ///
  /// Word boundaries are defined more precisely in Unicode Standard Annex #29
  /// <http://www.unicode.org/reports/tr29/#Word_Boundaries>.
  ///
  /// Valid only after [layout].
  TextRange getWordBoundary(TextPosition position) {
    assert(!debugNeedsLayout);
    _layoutTextWithConstraints(constraints);
    return _textPainter.getWordBoundary(position);
  }

  TextRange _getLineAtOffset(TextPosition position) => _textPainter.getLineBoundary(position);

  TextPosition _getTextPositionAbove(TextPosition position) {
    // -0.5 of preferredLineHeight points to the middle of the line above.
    final double preferredLineHeight = _textPainter.preferredLineHeight;
    final double verticalOffset = -0.5 * preferredLineHeight;
    return _getTextPositionVertical(position, verticalOffset);
  }

  TextPosition _getTextPositionBelow(TextPosition position) {
    // 1.5 of preferredLineHeight points to the middle of the line below.
    final double preferredLineHeight = _textPainter.preferredLineHeight;
    final double verticalOffset = 1.5 * preferredLineHeight;
    return _getTextPositionVertical(position, verticalOffset);
  }

  TextPosition _getTextPositionVertical(TextPosition position, double verticalOffset) {
    final Offset caretOffset = _textPainter.getOffsetForCaret(position, Rect.zero);
    final Offset caretOffsetTranslated = caretOffset.translate(0.0, verticalOffset);
    return _textPainter.getPositionForOffset(caretOffsetTranslated);
  }

  /// Returns the size of the text as laid out.
  ///
  /// This can differ from [size] if the text overflowed or if the [constraints]
  /// provided by the parent [RenderObject] forced the layout to be bigger than
  /// necessary for the given [text].
  ///
  /// This returns the [TextPainter.size] of the underlying [TextPainter].
  ///
  /// Valid only after [layout].
  Size get textSize {
    assert(!debugNeedsLayout);
    return _textPainter.size;
  }

  /// Whether the text was truncated or ellipsized as laid out.
  ///
  /// This returns the [TextPainter.didExceedMaxLines] of the underlying [TextPainter].
  ///
  /// Valid only after [layout].
  bool get didExceedMaxLines {
    assert(!debugNeedsLayout);
    return _textPainter.didExceedMaxLines;
  }
}
*/
