// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

// Package linebreak implements the Unicode Line Breaking Algorithm (UAX #14).
//
// It currently implements revision 53 of the algorithm, for Unicode 16.0.0.
package linebreak

import (
	"fmt"
	"math"
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

	SOT
	EOT
)

const (
	unprocessedBreak uint8 = iota
	alwaysBreak
	neverBreak
	mayBreak
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

	*word = (*word & ^(mask << shift)) | (uint64(v) << shift)
}

func (ins *Instance) Process(text []rune) []bool {
	if len(text) == 0 {
		return nil
	}

	before := newUint2s(len(text) + 1)
	for i := range before {
		// Initialize all values to mayBreak
		before[i] = math.MaxUint64
	}
	appliedRules := make([]uint16, len(text)+1)
	for i := range appliedRules {
		appliedRules[i] = 310
	}
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
		if i < 0 {
			return SOT
		}
		return runeClasses[i]
	}
	isAKASCircle := func(i int) bool {
		cls := class(i)
		return cls == AK || cls == AS || (i < len(runeClasses) && text[indices[i]] == '◌')
	}
	neverBreakBefore := func(i int, rule uint16) {
		i = indices[i]
		if rule < appliedRules[i] {
			before.set(i, neverBreak)
			appliedRules[i] = rule
		}
	}
	alwaysBreakBefore := func(i int, rule uint16) {
		i = indices[i]
		if rule < appliedRules[i] {
			before.set(i, alwaysBreak)
			appliedRules[i] = rule
		}
	}
	mayBreakBefore := func(i int, rule uint16) {
		i = indices[i]
		if rule < appliedRules[i] {
			before.set(i, mayBreak)
			appliedRules[i] = rule
		}
	}

	neverBreakAfter := func(i int, rule uint16) {
		if i+1 < len(runeClasses) {
			neverBreakBefore(i+1, rule)
		}
	}
	alwaysBreakAfter := func(i int, rule uint16) {
		if i+1 < len(runeClasses) {
			alwaysBreakBefore(i+1, rule)
		}
	}
	mayBreakAfter := func(i int, rule uint16) {
		if i+1 < len(runeClasses) {
			mayBreakBefore(i+1, rule)
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
			appliedRules[i+1] = 10
			fallthrough
		case CM:
			if catchCM {
				// LB9
				before.set(i, neverBreak)
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
	neverBreakBefore(0, 20)
	// LB3 - ! eot
	alwaysBreakAfter(len(text)-1, 30)

	// LB30a
	ris := 0
	for i, cls := range runeClasses {
		// LB30a
		if cls != RI {
			ris = 0
		} else if class(i+1) == RI {
			ris++
			if ris%2 != 0 {
				neverBreakAfter(i, 301)
			}
		}

		switch cls {
		case BK:
			// LB4 - BK !
			alwaysBreakAfter(i, 40)
			// LB6 - × [BK CR LF NL]
			neverBreakBefore(i, 60)

		case CR:
			// LB5 - CR × LF, [CR LF NL] !
			if class(i+1) == LF {
				neverBreakAfter(i, 50)
			} else {
				alwaysBreakAfter(i, 50)
			}
			// LB6 - × [BK CR LF NL]
			neverBreakBefore(i, 60)
		case LF, NL:
			// LB5 - CR × LF, [CR LF NL] !
			alwaysBreakAfter(i, 50)
			// LB6 - × [BK CR LF NL]
			neverBreakBefore(i, 60)

		case SP:
			// LB7 - × [SP ZW]
			neverBreakBefore(i, 70)
			// LB15c - SP ÷ IS NU
			if class(i+1) == IS && class(i+2) == NU {
				mayBreakAfter(i, 153)
			}
			// LB18 - SP ÷
			mayBreakAfter(i, 180)
		case ZW:
			// LB7 - × [SP ZW]
			neverBreakBefore(i, 70)
			// LB8 - ZW SP* ÷
			mayBreakAfter(findEndOfSPChain(i), 80)
		case ZWJ:
			// LB8a - ZWJ ×
			neverBreakAfter(i, 81)
		case WJ:
			// LB11 - × WJ, WJ ×
			neverBreakBefore(i, 110)
			neverBreakAfter(i, 110)
		case GL:
			// LB12 - GL ×
			neverBreakAfter(i, 120)

			// LB12a - [^SP BA HY] × GL
			switch class(i - 1) {
			case SP, BA, HY:
			default:
				neverBreakBefore(i, 121)
			}
		case OP:
			// LB14 - OP SP* ×
			neverBreakAfter(findEndOfSPChain(i), 140)
		case QU:
			r := text[indices[i]]

			// LB15a - [sot BK CR LF NL OP QU GL SP ZW] [\p{Pi}&QU] SP* ×
			if unicode.Is(unicode.Pi, r) {
				if i != 0 {
					switch runeClasses[i-1] {
					case BK, CR, LF, NL, OP, QU, GL, SP, ZW:
						neverBreakAfter(findEndOfSPChain(i), 151)
					}
				} else {
					neverBreakAfter(findEndOfSPChain(i), 151)
				}
			} else {
				// LB19 - × [QU - \p{Pi}], [QU - \p{Pf}] ×
				neverBreakBefore(i, 190)
			}

			// LB15b - × [\p{Pf}&QU] [SP GL WJ CL QU CP EX IS SY BK CR LF NL ZW eot]
			if unicode.Is(unicode.Pf, r) {
				if i != len(runeClasses)-1 {
					switch runeClasses[i+1] {
					case SP, GL, WJ, CL, QU, CP, EX, IS, SY, BK, CR, LF, NL, ZW:
						neverBreakBefore(i, 152)
					}
				} else {
					neverBreakBefore(i, 152)
				}
			} else {
				// LB19 - × [QU - \p{Pi}], [QU - \p{Pf}] ×
				neverBreakAfter(i, 190)
			}

			// LB19a
			if i > 0 {
				if !unicode.Is(eastAsian, text[indices[i-1]]) {
					neverBreakBefore(i, 191)
				}
			}
			if i == len(runeClasses)-1 || !unicode.Is(eastAsian, text[indices[i+1]]) {
				neverBreakBefore(i, 191)
			}
			if i+1 < len(runeClasses) && !unicode.Is(eastAsian, text[indices[i+1]]) {
				neverBreakAfter(i, 191)
			}
			if i == 0 || !unicode.Is(eastAsian, text[indices[i-1]]) {
				neverBreakAfter(i, 191)
			}
		case IS:
			// LB15d - × IS
			neverBreakBefore(i, 154)

			// LB29 - IS × [AL HL]
			switch class(i + 1) {
			case AL, HL:
				neverBreakAfter(i, 290)
			case NU:
				// LB25
				// NU [SY IS]* [CL CP]? × [PO PR]
				// NU [SY IS]* × NU
				// [PO PR] × (OP IS?)? NU
				// [HY IS] × NU
				neverBreakAfter(i, 250)
			}
		case B2:
			// LB17 - B2 SP* × B2
			i := findEndOfSPChain(i)
			if class(i+1) == B2 {
				neverBreakAfter(i, 170)
			}
		case CB:
			// LB20 - ÷ CB, CB ÷
			// TODO allow specifying per-object breaking behavior
			mayBreakBefore(i, 200)
			mayBreakAfter(i, 200)
		case HL:
			// LB21a - HL (HY | [ BA - $EastAsian ]) × [^HL]
			if cls == HL && i+2 < len(runeClasses) {
				if class(i+1) == HY || (class(i+1) == BA && !unicode.Is(eastAsian, text[indices[i+1]])) {
					if class(i+2) != HL {
						neverBreakAfter(i+1, 211)
					}
				}
			}

			switch class(i + 1) {
			case NU:
				// LB23 - [AL HL] × NU, NU × [AL HL]
				neverBreakAfter(i, 230)
			case PR, PO:
				// LB24 - [PR PO] × [AL HL], [AL HL] × [PR PO]
				neverBreakAfter(i, 240)
			case AL, HL:
				// LB28 - [AL HL] × [AL HL]
				neverBreakAfter(i, 280)
			case OP:
				if !unicode.Is(eastAsian, text[indices[i+1]]) {
					// LB30 [AL HL NU] × [OP-$EastAsian], [CP-$EastAsian] × [AL HL NU]
					neverBreakAfter(i, 300)
				}
			}

		case IN:
			// LB22 - × IN
			neverBreakBefore(i, 220)
		case SY:
			// LB21b - SY × HL
			if class(i+1) == HL {
				neverBreakAfter(i, 212)
			}

			// LB13 - × [CL CP EX SY]
			neverBreakBefore(i, 130)
		case CL:
			// LB13 - × [CL CP EX SY]
			neverBreakBefore(i, 130)

			// LB16 - [CL CP] SP* × NS
			i := findEndOfSPChain(i)
			if class(i+1) == NS {
				neverBreakAfter(i, 160)
			}
		case CP:
			// LB13 - × [CL CP EX SY]
			neverBreakBefore(i, 130)

			// LB16 - [CL CP] SP* × NS
			i := findEndOfSPChain(i)
			if class(i+1) == NS {
				neverBreakAfter(i, 160)
			}

			// LB30 [AL HL NU] × [OP-$EastAsian], [CP-$EastAsian] × [AL HL NU]
			if !unicode.Is(eastAsian, text[indices[i]]) {
				switch class(i + 1) {
				case AL, HL, NU:
					neverBreakAfter(i, 300)
				}
			}
		case EX:
			// LB13 - × [CL CP EX SY]
			neverBreakBefore(i, 130)

		case HY:
			// LB21 - × [BA HY NS], BB ×
			neverBreakBefore(i, 210)

			switch class(i + 1) {
			case NU:
				// LB25
				// NU [SY IS]* [CL CP]? × [PO PR]
				// NU [SY IS]* × NU
				// [PO PR] × (OP IS?)? NU
				// [HY IS] × NU
				neverBreakAfter(i, 250)
			case AL:
				// LB20a - [sot BK CR LF NL SP ZW CB GL] [HY \u2010] × AL
				if i != 0 {
					switch runeClasses[i-1] {
					case BK, CR, LF, NL, SP, ZW, CB, GL:
						neverBreakAfter(i, 201)
					}
				} else {
					neverBreakAfter(i, 201)
				}
			}

		case BA, NS:
			// LB21 - × [BA HY NS], BB ×
			neverBreakBefore(i, 210)
		case BB:
			// LB21 - × [BA HY NS], BB ×
			neverBreakAfter(i, 210)

		case AL:
			switch class(i + 1) {
			case NU:
				// LB23 - [AL HL] × NU, NU × [AL HL]
				neverBreakAfter(i, 230)
			case PR, PO:
				// LB24 - [PR PO] × [AL HL], [AL HL] × [PR PO]
				neverBreakAfter(i, 240)
			case AL, HL:
				// LB28 - [AL HL] × [AL HL]
				neverBreakAfter(i, 280)
			case OP:
				if !unicode.Is(eastAsian, text[indices[i+1]]) {
					// LB30 [AL HL NU] × [OP-$EastAsian], [CP-$EastAsian] × [AL HL NU]
					neverBreakAfter(i, 300)
				}
			}

		case NU:
			switch class(i + 1) {
			case AL, HL:
				// LB23 - [AL HL] × NU, NU × [AL HL]
				neverBreakAfter(i, 230)
			case OP:
				if !unicode.Is(eastAsian, text[indices[i+1]]) {
					// LB30 [AL HL NU] × [OP-$EastAsian], [CP-$EastAsian] × [AL HL NU]
					neverBreakAfter(i, 300)
				}
			}

			// LB25
			// NU [SY IS]* [CL CP]? × [PO PR]
			// NU [SY IS]* × NU
			// [PO PR] × (OP IS?)? NU
			// [HY IS] × NU
			j := i
			for j++; j < len(runeClasses) && (runeClasses[j] == SY || runeClasses[j] == IS); j++ {
			}
			j--
			switch class(j + 1) {
			case CL, CP:
				switch class(j + 2) {
				case PO, PR:
					neverBreakAfter(j+1, 250)
				}
			case PO, PR:
				neverBreakAfter(j, 250)
			case NU:
				neverBreakAfter(j, 250)
			}

		case PR:
			switch class(i + 1) {
			case ID, EB, EM:
				// LB23a - PR × [ID EB EM], [ID EB EM] × PO
				neverBreakAfter(i, 231)
			case AL, HL:
				// LB24 - [PR PO] × [AL HL], [AL HL] × [PR PO]
				neverBreakAfter(i, 240)
			case JL, JV, JT, H2, H3:
				// LB27
				// [JL JV JT H2 H3] × PO
				// PR × [JL JV JT H2 H3]
				//
				// TODO "When Korean uses SPACE for line breaking, the classes in rule LB26,
				// as well as characters of class ID, are often tailored to AL; see Section
				// 8, Customization."
				neverBreakAfter(i, 270)
			case OP:
				// LB25
				// NU [SY IS]* [CL CP]? × [PO PR]
				// NU [SY IS]* × NU
				// [PO PR] × (OP IS?)? NU
				// [HY IS] × NU
				if class(i+2) == NU || (class(i+2) == IS && class(i+3) == NU) {
					neverBreakAfter(i, 250)
				}
			case NU:
				// LB25
				// NU [SY IS]* [CL CP]? × [PO PR]
				// NU [SY IS]* × NU
				// [PO PR] × (OP IS?)? NU
				// [HY IS] × NU
				neverBreakAfter(i, 250)
			}

		case EB:
			// LB30b - EB × EM, [\p{Extended_Pictographic}&\p{Cn}] × EM
			if class(i+1) == EM {
				neverBreakAfter(i, 302)
			}
			// LB23a - PR × [ID EB EM], [ID EB EM] × PO
			fallthrough
		case ID, EM:
			// LB23a - PR × [ID EB EM], [ID EB EM] × PO
			if class(i+1) == PO {
				neverBreakAfter(i, 231)
			}

		case PO:
			switch class(i + 1) {
			case AL, HL:
				// LB24 - [PR PO] × [AL HL], [AL HL] × [PR PO]
				neverBreakAfter(i, 240)
			case OP:
				// LB25
				// NU [SY IS]* [CL CP]? × [PO PR]
				// NU [SY IS]* × NU
				// [PO PR] × (OP IS?)? NU
				// [HY IS] × NU
				if class(i+2) == NU || (class(i+2) == IS && class(i+3) == NU) {
					neverBreakAfter(i, 250)
				}
			case NU:
				// LB25
				// NU [SY IS]* [CL CP]? × [PO PR]
				// NU [SY IS]* × NU
				// [PO PR] × (OP IS?)? NU
				// [HY IS] × NU
				neverBreakAfter(i, 250)
			}

		case JL:
			switch class(i + 1) {
			case JL, JV, H2, H3:
				// LB26
				// JL × [JL JV H2 H3]
				// [JV H2] × [JV JT]
				// [JT H3] × JT
				neverBreakAfter(i, 260)
			case PO:
				// LB27
				// [JL JV JT H2 H3] × PO
				// PR × [JL JV JT H2 H3]
				//
				// TODO "When Korean uses SPACE for line breaking, the classes in rule LB26,
				// as well as characters of class ID, are often tailored to AL; see Section
				// 8, Customization."
				neverBreakAfter(i, 270)
			}
		case JV, H2:
			switch class(i + 1) {
			case JV, JT:
				// LB26
				// JL × [JL JV H2 H3]
				// [JV H2] × [JV JT]
				// [JT H3] × JT
				neverBreakAfter(i, 260)
			case PO:
				// LB27
				// [JL JV JT H2 H3] × PO
				// PR × [JL JV JT H2 H3]
				//
				// TODO "When Korean uses SPACE for line breaking, the classes in rule LB26,
				// as well as characters of class ID, are often tailored to AL; see Section
				// 8, Customization."
				neverBreakAfter(i, 270)
			}
		case JT, H3:
			switch class(i + 1) {
			case JT:
				// LB26
				// JL × [JL JV H2 H3]
				// [JV H2] × [JV JT]
				// [JT H3] × JT
				neverBreakAfter(i, 260)
			case PO:
				// LB27
				// [JL JV JT H2 H3] × PO
				// PR × [JL JV JT H2 H3]
				//
				// TODO "When Korean uses SPACE for line breaking, the classes in rule LB26,
				// as well as characters of class ID, are often tailored to AL; see Section
				// 8, Customization."
				neverBreakAfter(i, 270)
			}

		case AP:
			// LB28a
			// AP × [AK ◌ AS]
			// [AK ◌ AS] × [VF VI]
			// [AK ◌ AS] VI × [AK ◌]
			// [AK ◌ AS] × [AK ◌ AS] VF
			if isAKASCircle(i + 1) {
				neverBreakAfter(i, 281)
			}

		case AK, AS:
			// LB28a
			// AP × [AK ◌ AS]
			// [AK ◌ AS] × [VF VI]
			// [AK ◌ AS] VI × [AK ◌]
			// [AK ◌ AS] × [AK ◌ AS] VF
			switch class(i + 1) {
			case VI:
				if class(i+2) == AK || (i+2 < len(runeClasses) && text[indices[i+2]] == '◌') {
					neverBreakAfter(i+1, 281)
				}
				fallthrough
			case VF:
				neverBreakAfter(i, 281)
			case AK, AS:
				if class(i+2) == VF {
					neverBreakAfter(i, 281)
				}
			default:
				if i+1 < len(runeClasses) && text[indices[i+1]] == '◌' && class(i+2) == VF {
					neverBreakAfter(i, 281)
				}
			}
		}

		r := text[indices[i]]
		switch r {
		case '◌':
			// LB28a
			// AP × [AK ◌ AS]
			// [AK ◌ AS] × [VF VI]
			// [AK ◌ AS] VI × [AK ◌]
			// [AK ◌ AS] × [AK ◌ AS] VF
			switch class(i + 1) {
			case VI:
				if class(i+2) == AK || (i+2 < len(runeClasses) && text[indices[i+2]] == '◌') {
					neverBreakAfter(i+1, 281)
				}
				fallthrough
			case VF:
				neverBreakAfter(i, 281)
			case AK, AS:
				if class(i+2) == VF {
					neverBreakAfter(i, 281)
				}
			default:
				if i+1 < len(runeClasses) && text[indices[i+1]] == '◌' && class(i+2) == VF {
					neverBreakAfter(i, 281)
				}
			}
		case 0x2010:
			// LB20a - [sot BK CR LF NL SP ZW CB GL] [HY \u2010] × AL
			if class(i+1) == AL {
				if i != 0 {
					switch runeClasses[i-1] {
					case BK, CR, LF, NL, SP, ZW, CB, GL:
						neverBreakAfter(i, 201)
					}
				} else {
					neverBreakAfter(i, 201)
				}
			}
		}

		// LB30b - EB × EM, [\p{Extended_Pictographic}&\p{Cn}] × EM
		if class(i+1) == EM {
			if r := text[indices[i]]; unicode.Is(extendedPictographic, text[indices[i]]) {
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
					neverBreakAfter(i, 302)
				}
			}
		}
	}

	var out []bool
	for j, word := range before {
		for i := range 32 {
			switch uint8(word >> (2 * i) & 0b11) {
			case neverBreak:
				out = append(out, false)
			case alwaysBreak, mayBreak:
				out = append(out, true)
			case unprocessedBreak:
				if j*32+i < len(text) {
					panic("unreachable")
				}
			}
		}
	}
	if len(out) < len(text) {
		panic(fmt.Sprintf("internal error: produced %d values for %d runes", len(out), len(text)))
	}
	if len(out) == len(text) {
		out = append(out, true)
	} else {
		out[len(text)] = true
	}
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
