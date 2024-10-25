// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

// Package linebreak implements the Unicode Line Breaking Algorithm (UAX #14).
//
// It currently implements revision 53 of the algorithm, for Unicode 16.0.0.
package linebreak

import (
	"unicode"
)

//go:generate go run ./internal/cmd/generate_tables
//go:generate gofmt -w ./data.go

type Instance struct{}

type breakClass uint8

const (
	XX  breakClass = iota // Unknown
	BK                    // Mandatory Break
	CR                    // Carriage Return
	LF                    // Line Feed
	CM                    // Combining Mark
	NL                    // Next Line
	SG                    // Surrogate
	WJ                    // Word Joiner
	ZW                    // Zero Width Space
	GL                    // Non-breaking (“Glue”)
	SP                    // Space
	ZWJ                   // Zero Width Joiner
	B2                    // Break Opportunity Before and After
	BA                    // Break After
	BB                    // Break Before
	HY                    // Hyphen
	CB                    // Contingent Break Opportunity
	CL                    // Close Punctuation
	CP                    // Close Parenthesis
	EX                    // Exclamation/ Interrogation
	IN                    // Inseparable
	NS                    // Nonstarter
	OP                    // Open Punctuation
	QU                    // Quotation
	IS                    // Infix Numeric Separator
	NU                    // Numeric
	PO                    // Postfix Numeric
	PR                    // Prefix Numeric
	SY                    // Symbols Allowing Break After
	AI                    // Ambiguous (Alphabetic or Ideographic)
	AK                    // Aksara
	AL                    // Alphabetic
	AP                    // Aksara Pre-Base
	AS                    // Aksara Start
	CJ                    // Conditional Japanese Starter
	EB                    // Emoji Base
	EM                    // Emoji Modifier
	H2                    // Hangul LV Syllable
	H3                    // Hangul LVT Syllable
	HL                    // Hebrew Letter
	ID                    // Ideographic
	JL                    // Hangul L Jamo
	JV                    // Hangul V Jamo
	JT                    // Hangul T Jamo
	RI                    // Regional Indicator
	SA                    // Complex Context Dependent (South East Asian)
	VF                    // Virama Final
	VI                    // Virama

	EOT
)

const (
	unprocessedBreak uint8 = iota
	alwaysBreak
	mayBreak
	neverBreak
)

type uint2s []uint64

func newUint2s(n int) uint2s {
	return make([]uint64, (2*n+63)/64)
}

func (bs uint2s) get(idx int) uint8 {
	const wordSize = 64
	const bits = 2
	const mask = (1 << bits) - 1

	word := bs[uint(idx)/(wordSize/bits)]
	shift := (uint(idx) % (wordSize / bits)) * bits
	ret := uint8((word >> shift) & mask)
	return ret
}

func (bs uint2s) set(idx int, v uint8) {
	const wordSize = 64
	const bits = 2
	const mask = (1 << bits) - 1

	v = v & mask
	word := &bs[uint(idx)/(wordSize/bits)]
	shift := (uint(idx) % (wordSize / bits)) * bits
	*word |= uint64(v) << shift
}

func (ins *Instance) Process(text []rune) []bool {
	if len(text) == 0 {
		return nil
	}

	// UAX #14 specifies rules as "(do not) break before/after", but breaking
	// after i is the same as breaking before i+1, so we don't have to track
	// before and after separately. The only exception is "do not break after
	// the end of text", because we do not have an i+1 for that. However, we can
	// trivially handle that rule specially.

	before := newUint2s(len(text) + 1)
	// indices contains indices into text of runes we aren't skipping
	indices := make([]int, 0, len(text))
	// runeClasses contains classes for runes we aren't skipping. runeClasses[i]
	// is the class of text[indices[i]].
	runeClasses := make([]breakClass, 0, len(text))

	findEndOfSPChain := func(i int) int {
		if i == len(runeClasses)-1 {
			return i
		}
		j := i + 1
		for ; j < len(runeClasses) && runeClasses[j] == SP; j++ {
		}
		return j - 1
	}

	class := func(i int) breakClass {
		if i >= len(runeClasses) {
			return EOT
		}
		return runeClasses[i]
	}
	neverBreakBefore := func(i int) {
		i = indices[i]
		if before.get(i) == unprocessedBreak {
			before.set(i, neverBreak)
		}
	}
	alwaysBreakBefore := func(i int) {
		i = indices[i]
		if before.get(i) == unprocessedBreak {
			before.set(i, alwaysBreak)
		}
	}
	mayBreakBefore := func(i int) {
		i = indices[i]
		if before.get(i) == unprocessedBreak {
			before.set(i, mayBreak)
		}
	}

	neverBreakAfter := func(i int) {
		if i+1 < len(runeClasses) {
			neverBreakBefore(i + 1)
		}
	}
	alwaysBreakAfter := func(i int) {
		if i+1 < len(runeClasses) {
			alwaysBreakBefore(i + 1)
		}
	}
	mayBreakAfter := func(i int) {
		if i+1 < len(runeClasses) {
			mayBreakBefore(i + 1)
		}
	}

	// LB1
	catchCM := false
	for i, r := range text {
		cls := runeClass(r)

		// TODO allow tailoring this behavior.
		switch cls {
		case AI, SG, XX:
			// TODO for AI should we take East_Asian_Width into consideration?
			cls = AL
		case SA:
			// TODO support dictionary-based morphological analysis
			if unicode.In(r, unicode.Mn, unicode.Mc) {
				cls = CM
			} else {
				cls = AL
			}
		case CJ:
			cls = NS
		}

		switch cls {
		case BK, CR, LF, NL, SP, ZW:
			indices = append(indices, i)
			catchCM = false
			runeClasses = append(runeClasses, cls)
		case ZWJ:
			before.set(i+1, neverBreak)
			fallthrough
		case CM:
			if catchCM {
				// LB9
				continue
			} else {
				// LB10
				indices = append(indices, i)
				runeClasses = append(runeClasses, AL)
			}
		default:
			indices = append(indices, i)
			runeClasses = append(runeClasses, cls)
			catchCM = true
		}
	}
	if catchCM {
		indices = append(indices, len(text))
	}

	// LB2 - sot ×
	neverBreakBefore(0)
	// LB3 - ! eot
	alwaysBreakAfter(len(text) - 1)

	// OPT try to merge as many of these rules as possible

	// Note that we cannot trivially apply all rules in a single pass over the
	// text. Some rules affect the previous rune but have priority over later
	// rules. For example, LB15d is '× IS' and forbids breaking before ';', ',',
	// and '.' even after spaces. LB18, however, is 'SP ÷', which allows
	// breaking after spaces. If we apply all rules per rune, we'll first see
	// the space, apply LB18, then see the IS and fail to apply LB15d because
	// we've already allowed the break.
	//
	// However, we cannot categorically allow decisions to override each other,
	// either, or any sequence of rules will not work correctly. For example, if
	// a hypothetical rule N1 has 'X !', but N2 has '× Y', then 'X Y' is meant
	// to allow a break between X and Y, because N1 prevails.
	//
	// TODO Maybe we can work around this by tracking which rule made a
	// decision, and allow previous rules to override decisions made by later
	// rules?

	// LB4 - BK !
	for i, cls := range runeClasses {
		if cls == BK {
			alwaysBreakAfter(i)
		}
	}

	// LB5 - CR × LF, [CR LF NL] !
	for i, cls := range runeClasses {
		switch cls {
		case CR:
			if class(i+1) == LF {
				neverBreakAfter(i)
			} else {
				alwaysBreakAfter(i)
			}
		case LF, NL:
			alwaysBreakAfter(i)
		}
	}

	// LB6 - × [BK CR LF NL]
	for i, cls := range runeClasses {
		switch cls {
		case BK, CR, LF, NL:
			neverBreakBefore(i)
		}
	}

	// LB7 - × [SP ZW]
	for i, cls := range runeClasses {
		switch cls {
		case SP, ZW:
			neverBreakBefore(i)
		}
	}

	// LB8 - ZW SP* ÷
	for i, cls := range runeClasses {
		if cls == ZW {
			mayBreakAfter(findEndOfSPChain(i))
		}
	}

	// LB8a - ZWJ ×
	for i, cls := range runeClasses {
		if cls == ZWJ {
			neverBreakAfter(i)
		}
	}

	// LB11 - × WJ, WJ ×
	for i, cls := range runeClasses {
		if cls == WJ {
			neverBreakBefore(i)
			neverBreakAfter(i)
		}
	}

	// LB12 - GL ×
	for i, cls := range runeClasses {
		if cls == GL {
			neverBreakAfter(i)
		}
	}

	// LB12a - [^SP BA HY] × GL
	for i, cls := range runeClasses {
		switch cls {
		case SP, BA, HY:
		default:
			if class(i+1) == GL {
				neverBreakAfter(i)
			}
		}
	}

	// LB13 - × [CL CP EX SY]
	for i, cls := range runeClasses {
		switch cls {
		case CL, CP, EX, SY:
			neverBreakBefore(i)
		}
	}

	// LB14 - OP SP* ×
	for i, cls := range runeClasses {
		if cls == OP {
			neverBreakAfter(findEndOfSPChain(i))
		}
	}

	// LB15a - [sot BK CR LF NL OP QU GL SP ZW] [\p{Pi}&QU] SP* ×
	for i, cls := range runeClasses {
		r := text[indices[i]]
		if cls != QU || !unicode.Is(unicode.Pi, r) {
			continue
		}
		if i != 0 {
			switch runeClasses[i-1] {
			case BK, CR, LF, NL, OP, QU, GL, SP, ZW:
			default:
				continue
			}
		}
		neverBreakAfter(findEndOfSPChain(i))
	}

	// LB15b - × [\p{Pf}&QU] [SP GL WJ CL QU CP EX IS SY BK CR LF NL ZW eot]
	for i, cls := range runeClasses {
		r := text[indices[i]]
		if cls != QU || !unicode.Is(unicode.Pf, r) {
			continue
		}
		if i != len(runeClasses)-1 {
			switch runeClasses[i+1] {
			case SP, GL, WJ, CL, QU, CP, EX, IS, SY, BK, CR, LF, NL, ZW:
			default:
				continue
			}
		}
		neverBreakBefore(i)
	}

	// LB15c - SP ÷ IS NU
	for i, cls := range runeClasses {
		if cls == SP && class(i+1) == IS && class(i+2) == NU {
			mayBreakAfter(i)
		}
	}

	// LB15d - × IS
	for i, cls := range runeClasses {
		if cls == IS {
			neverBreakBefore(i)
		}
	}

	// LB16 - [CL CP] SP* × NS
	for i, cls := range runeClasses {
		switch cls {
		case CL, CP:
			i = findEndOfSPChain(i)
			if class(i+1) == NS {
				neverBreakAfter(i)
			}
		}
	}

	// LB17 - B2 SP* × B2
	for i, cls := range runeClasses {
		if cls == B2 {
			i = findEndOfSPChain(i)
			if class(i+1) == B2 {
				neverBreakAfter(i)
			}
		}
	}

	// LB18 - SP ÷
	for i, cls := range runeClasses {
		if cls == SP {
			mayBreakAfter(i)
		}
	}

	// LB19 - × [QU - \p{Pi}], [QU - \p{Pf}] ×
	for i, cls := range runeClasses {
		r := text[indices[i]]
		if cls == QU {
			if !unicode.Is(unicode.Pi, r) {
				neverBreakBefore(i)
			}
			if !unicode.Is(unicode.Pf, r) {
				neverBreakAfter(i)
			}
		}
	}

	// LB19a
	for i, cls := range runeClasses {
		if cls != QU {
			continue
		}
		if i > 0 {
			if !unicode.Is(eastAsian, text[indices[i-1]]) {
				neverBreakBefore(i)
			}
		}
		if i == len(runeClasses)-1 || !unicode.Is(eastAsian, text[indices[i+1]]) {
			neverBreakBefore(i)
		}
		if i+1 < len(runeClasses) && !unicode.Is(eastAsian, text[indices[i+1]]) {
			neverBreakAfter(i)
		}
		if i == 0 || !unicode.Is(eastAsian, text[indices[i-1]]) {
			neverBreakAfter(i)
		}
	}

	// LB20 - ÷ CB, CB ÷
	// TODO allow specifying per-object breaking behavior
	for i, cls := range runeClasses {
		if cls == CB {
			mayBreakBefore(i)
			mayBreakAfter(i)
		}
	}

	// LB20a - [sot BK CR LF NL SP ZW CB GL] [HY \u2010] × AL
	for i, cls := range runeClasses {
		if cls != HY && text[indices[i]] != 0x2010 {
			continue
		}
		if i != 0 {
			switch runeClasses[i-1] {
			case BK, CR, LF, NL, SP, ZW, CB, GL:
			default:
				continue
			}
		}
		if class(i+1) == AL {
			neverBreakAfter(i)
		}
	}

	// LB21 - × [BA HY NS], BB ×
	for i, cls := range runeClasses {
		switch cls {
		case BA, HY, NS:
			neverBreakBefore(i)
		case BB:
			neverBreakAfter(i)
		}
	}

	// LB21a - HL (HY | [ BA - $EastAsian ]) × [^HL]
	for i, cls := range runeClasses {
		if cls != HL || i+2 >= len(runeClasses) {
			continue
		}

		if class(i+1) != HY && (class(i+1) != BA || unicode.Is(eastAsian, text[indices[i+1]])) {
			continue
		}
		if class(i+2) == HL {
			continue
		}
		neverBreakAfter(i + 1)
	}

	// LB21b - SY × HL
	for i, cls := range runeClasses {
		if cls == SY && class(i+1) == HL {
			neverBreakAfter(i)
		}
	}

	// LB22 - × IN
	for i, cls := range runeClasses {
		if cls == IN {
			neverBreakBefore(i)
		}
	}

	// LB23 - [AL HL] × NU, NU × [AL HL]
	for i, cls := range runeClasses {
		switch cls {
		case AL, HL:
			if class(i+1) == NU {
				neverBreakAfter(i)
			}
		case NU:
			switch class(i + 1) {
			case AL, HL:
				neverBreakAfter(i)
			}
		}
	}

	// LB23a - PR × [ID EB EM], [ID EB EM] × PO
	for i, cls := range runeClasses {
		switch cls {
		case PR:
			switch class(i + 1) {
			case ID, EB, EM:
				neverBreakAfter(i)
			}
		case ID, EB, EM:
			if class(i+1) == PO {
				neverBreakAfter(i)
			}
		}
	}

	// LB24 - [PR PO] × [AL HL], [AL HL] × [PR PO]
	for i, cls := range runeClasses {
		switch cls {
		case PR, PO:
			switch class(i + 1) {
			case AL, HL:
				neverBreakAfter(i)
			}
		case AL, HL:
			switch class(i + 1) {
			case PR, PO:
				neverBreakAfter(i)
			}
		}
	}

	// TODO "In general, it is recommended to not break lines inside numbers of
	// the form described by the following regular expression: ( PR | PO) ? ( OP
	// | HY ) ? IS ? NU (NU | SY | IS) * (CL | CP) ? ( PR | PO) ?"

	// LB25
	// NU [SY IS]* [CL CP]? × [PO PR]
	// NU [SY IS]* × NU
	// [PO PR] × (OP IS?)? NU
	// [HY IS] × NU
	for i, cls := range runeClasses {
		switch cls {
		case NU:
			for i++; i < len(runeClasses) && (runeClasses[i] == SY || runeClasses[i] == IS); i++ {
			}
			i--
			switch class(i + 1) {
			case CL, CP:
				switch class(i + 2) {
				case PO, PR:
					neverBreakAfter(i + 1)
				}
			case PO, PR:
				neverBreakAfter(i)
			case NU:
				neverBreakAfter(i)
			}
		case PO, PR:
			switch class(i + 1) {
			case OP:
				if class(i+2) == NU || (class(i+2) == IS && class(i+3) == NU) {
					neverBreakAfter(i)
				}
			case NU:
				neverBreakAfter(i)
			}
		case HY, IS:
			if class(i+1) == NU {
				neverBreakAfter(i)
			}
		}
	}

	// LB26
	// JL × [JL JV H2 H3]
	// [JV H2] × [JV JT]
	// [JT H3] × JT
	for i, cls := range runeClasses {
		switch cls {
		case JL:
			switch class(i + 1) {
			case JL, JV, H2, H3:
				neverBreakAfter(i)
			}
		case JV, H2:
			switch class(i + 1) {
			case JV, JT:
				neverBreakAfter(i)
			}
		case JT, H3:
			if class(i+1) == JT {
				neverBreakAfter(i)
			}
		}
	}

	// LB27
	// [JL JV JT H2 H3] × PO
	// PR × [JL JV JT H2 H3]
	//
	// TODO "When Korean uses SPACE for line breaking, the classes in rule LB26,
	// as well as characters of class ID, are often tailored to AL; see Section
	// 8, Customization."
	for i, cls := range runeClasses {
		switch cls {
		case JL, JV, JT, H2, H3:
			if class(i+1) == PO {
				neverBreakAfter(i)
			}
		case PR:
			switch class(i + 1) {
			case JL, JV, JT, H2, H3:
				neverBreakAfter(i)
			}
		}
	}

	// LB28 - [AL HL] × [AL HL]
	for i, cls := range runeClasses {
		switch cls {
		case AL, HL:
			switch class(i + 1) {
			case AL, HL:
				neverBreakAfter(i)
			}
		}
	}

	// LB28a
	// AP × [AK ◌ AS]
	// [AK ◌ AS] × [VF VI]
	// [AK ◌ AS] VI × [AK ◌]
	// [AK ◌ AS] × [AK ◌ AS] VF
	isAKASCircle := func(i int) bool {
		if i >= len(runeClasses) {
			return false
		}
		cls := class(i)
		return cls == AK || cls == AS || text[indices[i]] == '◌'
	}
	for i, cls := range runeClasses {
		if cls == AP {
			if isAKASCircle(i + 1) {
				neverBreakAfter(i)
			}
		} else if isAKASCircle(i) {
			if class(i+1) == VI {
				neverBreakAfter(i)
				if class(i+2) == AK || (i+2 < len(runeClasses) && text[indices[i+2]] == '◌') {
					neverBreakAfter(i + 1)
				}
			} else if class(i+1) == VF {
				neverBreakAfter(i)
			} else if isAKASCircle(i + 1) {
				if class(i+2) == VF {
					neverBreakAfter(i)
				}
			}
		}
	}

	// LB29 - IS × [AL HL]
	for i, cls := range runeClasses {
		if cls == IS {
			switch class(i + 1) {
			case AL, HL:
				neverBreakAfter(i)
			}
		}
	}

	// LB30 [AL HL NU] × [OP-$EastAsian], [CP-$EastAsian] × [AL HL NU]
	for i, cls := range runeClasses {
		switch cls {
		case AL, HL, NU:
			if i+1 < len(runeClasses) && class(i+1) == OP && !unicode.Is(eastAsian, text[indices[i+1]]) {
				neverBreakAfter(i)
			}
		case CP:
			if unicode.Is(eastAsian, text[indices[i]]) {
				continue
			}
			switch class(i + 1) {
			case AL, HL, NU:
				neverBreakAfter(i)
			}
		}
	}

	// LB30a
	ris := 0
	for i, cls := range runeClasses {
		if cls != RI {
			ris = 0
			continue
		}
		if cls == RI && class(i+1) == RI {
			ris++
			if ris%2 != 0 {
				neverBreakAfter(i)
			}
		}
	}

	// LB30b - EB × EM, [\p{Extended_Pictographic}&\p{Cn}] × EM
	for i, cls := range runeClasses {
		if class(i+1) != EM {
			continue
		}

		if cls == EB {
			neverBreakAfter(i)
		} else if r := text[indices[i]]; unicode.Is(extendedPictographic, text[indices[i]]) {
			// OPT hopefully we don't get here often, because that check won't
			// be very fast.
			if !unicode.In(
				r,
				unicode.C,
				unicode.L,
				unicode.M,
				unicode.N,
				unicode.P,
				unicode.S,
				unicode.Z,
			) {
				neverBreakAfter(i)
			}
		}
	}

	// LB31
	for _, idx := range indices {
		if before.get(idx) == unprocessedBreak {
			before.set(idx, mayBreak)
		}
	}

	var out []bool
	for _, word := range before {
		for i := range 32 {
			switch uint8(word >> (2 * i) & 0b11) {
			case neverBreak:
				out = append(out, false)
			case alwaysBreak, mayBreak:
				out = append(out, true)
			case unprocessedBreak:
				out = append(out, false)
				// panic("unreachable")
			}
		}
	}
	out[len(text)] = true
	return out[:len(text)+1]
}

func runeClass(r rune) breakClass {
	// Valid Unicode code points are at most 21 bits long. The low 8 bits are
	// used as the offset in a block, leaving us with 13 bits for looking up the
	// block index. Masking with 0x1FFF lets the compiler elide the bounds
	// check, although it means we'll return invalid data for invalid runes.
	blockID := index[(r>>8)&0x1FFF]
	blockOff := uint(blockID)<<8 + uint(r&0xFF)
	// This doesn't emit a bounds check because blockID is uint8, which means
	// blockOff can be at most 256*255, which is less than len(data). (Though it
	// seems important that len(data) is at least 256*256.)
	v := data[blockOff]
	return breakClass(v)
}
