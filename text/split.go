// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package text

import (
	"cmp"
	"fmt"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/fontdb"
	xlanguage "honnef.co/go/gutter/internal/language"
	"honnef.co/go/gutter/internal/tinylfu"
	"honnef.co/go/gutter/opentype"
	"honnef.co/go/gutter/text/bidi"

	"github.com/go-text/typesetting/language"
)

// lookupDelimIndex binary searches in the list of the paired delimiters,
// and returns -1 if `ch` is not found
func lookupDelimIndex(ch rune) int {
	lower := 0
	upper := len(pairedDelims) - 1

	for lower <= upper {
		mid := (lower + upper) / 2

		if ch < pairedDelims[mid] {
			upper = mid - 1
		} else if ch > pairedDelims[mid] {
			lower = mid + 1
		} else {
			return mid
		}
	}

	return -1
}

var pairedDelims = [...]rune{
	0x0028, 0x0029,
	0x003c, 0x003e,
	0x005b, 0x005d,
	0x007b, 0x007d,
	0x00ab, 0x00bb,
	0x2018, 0x2019,
	0x201a, 0x201b,
	0x201c, 0x201d,
	0x201e, 0x201f,
	0x2039, 0x203a,
	0x2045, 0x2046,
	0x207d, 0x207e,
	0x208d, 0x208e,
	0x2308, 0x2309,
	0x230a, 0x230b,
	0x2329, 0x232a,
	0x2768, 0x2769,
	0x276a, 0x276b,
	0x276c, 0x276d,
	0x276e, 0x276f,
	0x2770, 0x2771,
	0x2772, 0x2773,
	0x2774, 0x2775,
	0x27c5, 0x27c6,
	0x27e6, 0x27e7,
	0x27e8, 0x27e9,
	0x27ea, 0x27eb,
	0x27ec, 0x27ed,
	0x27ee, 0x27ef,
	0x2983, 0x2984,
	0x2985, 0x2986,
	0x2987, 0x2988,
	0x2989, 0x298a,
	0x298b, 0x298c,
	0x298d, 0x298e,
	0x298f, 0x2990,
	0x2991, 0x2992,
	0x2993, 0x2994,
	0x2995, 0x2996,
	0x2997, 0x2998,
	0x29d8, 0x29d9,
	0x29da, 0x29db,
	0x29fc, 0x29fd,
	0x2e02, 0x2e03,
	0x2e04, 0x2e05,
	0x2e09, 0x2e0a,
	0x2e0c, 0x2e0d,
	0x2e1c, 0x2e1d,
	0x2e20, 0x2e21,
	0x2e22, 0x2e23,
	0x2e24, 0x2e25,
	0x2e26, 0x2e27,
	0x2e28, 0x2e29,
	0x2e42, 0x2e55,
	0x2e56, 0x2e57,
	0x2e58, 0x2e59,
	0x2e5a, 0x2e5b,
	0x2e5c, 0x3008,
	0x3009, 0x300a,
	0x300b, 0x300c,
	0x300d, 0x300e,
	0x300f, 0x3010,
	0x3011, 0x3014,
	0x3015, 0x3016,
	0x3017, 0x3018,
	0x3019, 0x301a,
	0x301b, 0x301d,
	0x301e, 0x301f,
	0xfd3e, 0xfd3f,
	0xfe17, 0xfe18,
	0xfe35, 0xfe36,
	0xfe37, 0xfe38,
	0xfe39, 0xfe3a,
	0xfe3b, 0xfe3c,
	0xfe3d, 0xfe3e,
	0xfe3f, 0xfe40,
	0xfe41, 0xfe42,
	0xfe43, 0xfe44,
	0xfe47, 0xfe48,
	0xfe59, 0xfe5a,
	0xfe5b, 0xfe5c,
	0xfe5d, 0xfe5e,
	0xff08, 0xff09,
	0xff3b, 0xff3d,
	0xff5b, 0xff5d,
	0xff5f, 0xff60,
	0xff62, 0xff63,
}

type delimEntry struct {
	index  int              // in the [pairedDelims] list
	script xlanguage.Script // resolved from the context
}

func (pb *ParagraphBuilder) splitByBidi() {
	// XXX the bidi package processes one paragraph at a time. We need to split
	// the text into paragraphs first.
	ins := bidi.Instance{
		ParagraphDirection: pb.style.Direction,
	}

	p := ins.Process(pb.text.runes)
	pb.text.bidiParagraph = p
	oldRuns := pb.text.runs
	bidiRuns := p.Runs()

	newRuns := make([]run, 0, max(len(oldRuns), len(bidiRuns)))
	var i, j int
	for i < len(oldRuns) && j < len(bidiRuns) {
		one := oldRuns[i]
		two := bidiRuns[j]

		newRun := one
		newRun.Run = bidi.Run{
			Start: max(one.Start, two.Start),
			End:   min(one.End, two.End),
			Level: two.Level,
		}
		newRun.runeStyles = newRun.runeStyles[newRun.Start-one.Start:]
		newRuns = append(newRuns, newRun)
		switch cmp.Compare(one.End, two.End) {
		case -1:
			i++
		case 1:
			j++
		case 0:
			i++
			j++
		}
	}
	if i != len(pb.text.runs) || j != len(bidiRuns) {
		panic(fmt.Sprintf("internal error: (%d, %d) != (%d, %d)",
			i, j, len(pb.text.runs), len(bidiRuns)))
	}

	pb.text.runs = newRuns
}

// See https://unicode.org/reports/tr24/#Common for reference
func (pb *ParagraphBuilder) splitByScript() {
	// OPT(dh): don't do this once per call to splitByScript
	common := xlanguage.MustParseScript("Zyyy")
	inherited := xlanguage.MustParseScript("Zinh")
	var delimStack []delimEntry

	type scriptRun struct {
		start, end int
		script     xlanguage.Script
	}

	// XXX don't crash on empty text
	scriptRuns := make([]scriptRun, 1, len(pb.text.runs))
	script := common
	for i, r := range pb.text.runes {
		// OPT(dh): this way of converting between language.Script and
		// xlanguage.Script is ridiculous.
		//
		// TODO look into generating a trie for the script property
		rScript := xlanguage.MustParseScript(language.LookupScript(r).String())

		// to properly handle Common script,
		// we register paired delimiters

		delimIndex := -1
		if rScript == common || rScript == inherited {
			delimIndex = lookupDelimIndex(r)
		}

		if delimIndex >= 0 { // handle paired characters
			if delimIndex%2 == 0 {
				// this is an open character : push it onto the stack
				delimStack = append(delimStack, delimEntry{delimIndex, script})
			} else {
				// this is a close character : try to look backward in the stack
				// for its counterpart
				counterPartIndex := delimIndex - 1
				j := len(delimStack) - 1
				for ; j >= 0; j-- {
					if delimStack[j].index == counterPartIndex { // found a match, use its script
						rScript = delimStack[j].script
						break
					}
				}
				// in any case, pop the open characters
				if j == -1 {
					j = 0
				}
				delimStack = delimStack[:j]
			}
		}

		// check if we have a 'real' change of script, or not
		if rScript == common || rScript == inherited || rScript == script {
			// no change
		} else if script == common {
			// update the pair stack to attribute the resolved script
			for i := range delimStack {
				delimStack[i].script = rScript
			}
			// set the resolved script to the current run,
			// but do NOT create a new run
			script = rScript
			scriptRuns[len(scriptRuns)-1].script = rScript
		} else {
			scriptRuns[len(scriptRuns)-1].end = i
			scriptRuns = append(scriptRuns, scriptRun{start: i, script: rScript})
			script = rScript
		}
	}
	scriptRuns[len(scriptRuns)-1].end = len(pb.text.runes)

	newRuns := make([]run, 0, max(len(pb.text.runs), len(scriptRuns)))
	var i, j int
	for i < len(pb.text.runs) && j < len(scriptRuns) {
		one := pb.text.runs[i]
		two := scriptRuns[j]
		newRun := one
		newRun.script = two.script
		newRun.Run = bidi.Run{
			Start: max(one.Start, two.start),
			End:   min(one.End, two.end),
			Level: one.Level,
		}
		newRun.runeStyles = newRun.runeStyles[newRun.Start-one.Start:]
		newRuns = append(newRuns, newRun)
		switch cmp.Compare(one.End, two.end) {
		case -1:
			i++
		case 1:
			j++
		case 0:
			i++
			j++
		}
	}
	if i != len(pb.text.runs) || j != len(scriptRuns) {
		panic("unreachable")
	}
	pb.text.runs = newRuns
}

func (pb *ParagraphBuilder) splitByFont(fdb *fontdb.Faces, fl *FontLoader) {
	// FIXME splitByFont isn't doing any splitting right now because we don't
	// check rune coverage.

	for i := range pb.text.runs {
		run := &pb.text.runs[i]

		// xlang := run.runStyle.Language.UnwrapOr(xlanguage.Und)
		// // XXX use system fonts, integrate with some font db, etc
		// ffs := NotoFonts

		// XXX allow explicit FontFamily in style

		query := make(map[opentype.Tag]float64)
		switch fs := run.runStyle.FontStyle.UnwrapOr(FontStyleNormal); fs {
		case FontStyleNormal:
			query["ital"] = 0
		case FontStyleItalic:
			query["ital"] = 1
		default:
			panic(fmt.Sprintf("unhandled font style %v", fs))
		}
		query["wght"] = run.runStyle.FontWeight.UnwrapOr(400)
		query["wdth"] = run.runStyle.FontWidth.UnwrapOr(100)

		// OPT don't allocate the default value repeatedly
		candidateNames := run.runStyle.FontFamilies.UnwrapOr([]string{"generic(system-ui)"})

		// FIXME(dh):
		// https://faultlore.com/blah/text-hates-you/#text-isnt-individual-characters
		// suggests that we should check an entire extended grapheme cluster
		// at a time and find a font that supports all runes in a cluster.
		for _, c := range candidateNames {
			fip, ok := fdb.Match(c, query)
			if !ok {
				continue
			}
			// XXX actually check that the font covers the rune
			// XXX pass the right index once we support font collections
			face := Face{Path: fip.Font.Path, Index: 0}
			key := makeFontKey(face, fip.AxisValues)
			font, ok := pb.fonts[key]
			if !ok {
				font = &Font{
					// XXX figure out when to garbage collect the font
					hb:         fl.Font(face, fip.AxisValues),
					glyphCache: tinylfu.New[int32, curve.BezPath](1024, 1024*10),
				}
				pb.fonts[key] = font
			}
			run.font = font
			break
		}

		if run.font == nil {
			// XXX
			panic("no font found")
		}
	}
}
