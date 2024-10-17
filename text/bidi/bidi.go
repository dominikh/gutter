// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package bidi

// References:
//
// https://unicode.org/reports/tr9/
// https://unicode.org/notes/tn39/

// OPT(dh): text contains long subsequences of the same character class. for
// example, in this comment, most characters are of class L. The bidi rules
// require us to repeatedly look at every rune in the text and look for a small
// set of control characters. Instead of scanning the entire text many times
// over, compute these subsequences and skip over entire subsequences at a time.
//
// This is complicated by the fact that rules update classes, requiring changes
// to existing runs.

import (
	"cmp"
	"fmt"
	"iter"
	"slices"

	"honnef.co/go/gutter/debug"

	"golang.org/x/text/unicode/bidi"
)

const trace = false

// OPT this information is already part of the trie, but x/text/unicode/bidi
// doesn't let us access it.
var bracketMirrors = map[rune]rune{
	0x0028: 0x0029, 0x0029: 0x0028, 0x005B: 0x005D, 0x005D: 0x005B,
	0x007B: 0x007D, 0x007D: 0x007B, 0x0F3A: 0x0F3B, 0x0F3B: 0x0F3A,
	0x0F3C: 0x0F3D, 0x0F3D: 0x0F3C, 0x169B: 0x169C, 0x169C: 0x169B,
	0x2045: 0x2046, 0x2046: 0x2045, 0x207D: 0x207E, 0x207E: 0x207D,
	0x208D: 0x208E, 0x208E: 0x208D, 0x2308: 0x2309, 0x2309: 0x2308,
	0x230A: 0x230B, 0x230B: 0x230A, 0x2329: 0x232A, 0x232A: 0x2329,
	0x2768: 0x2769, 0x2769: 0x2768, 0x276A: 0x276B, 0x276B: 0x276A,
	0x276C: 0x276D, 0x276D: 0x276C, 0x276E: 0x276F, 0x276F: 0x276E,
	0x2770: 0x2771, 0x2771: 0x2770, 0x2772: 0x2773, 0x2773: 0x2772,
	0x2774: 0x2775, 0x2775: 0x2774, 0x27C5: 0x27C6, 0x27C6: 0x27C5,
	0x27E6: 0x27E7, 0x27E7: 0x27E6, 0x27E8: 0x27E9, 0x27E9: 0x27E8,
	0x27EA: 0x27EB, 0x27EB: 0x27EA, 0x27EC: 0x27ED, 0x27ED: 0x27EC,
	0x27EE: 0x27EF, 0x27EF: 0x27EE, 0x2983: 0x2984, 0x2984: 0x2983,
	0x2985: 0x2986, 0x2986: 0x2985, 0x2987: 0x2988, 0x2988: 0x2987,
	0x2989: 0x298A, 0x298A: 0x2989, 0x298B: 0x298C, 0x298C: 0x298B,
	0x298D: 0x2990, 0x298E: 0x298F, 0x298F: 0x298E, 0x2990: 0x298D,
	0x2991: 0x2992, 0x2992: 0x2991, 0x2993: 0x2994, 0x2994: 0x2993,
	0x2995: 0x2996, 0x2996: 0x2995, 0x2997: 0x2998, 0x2998: 0x2997,
	0x29D8: 0x29D9, 0x29D9: 0x29D8, 0x29DA: 0x29DB, 0x29DB: 0x29DA,
	0x29FC: 0x29FD, 0x29FD: 0x29FC, 0x2E22: 0x2E23, 0x2E23: 0x2E22,
	0x2E24: 0x2E25, 0x2E25: 0x2E24, 0x2E26: 0x2E27, 0x2E27: 0x2E26,
	0x2E28: 0x2E29, 0x2E29: 0x2E28, 0x2E55: 0x2E56, 0x2E56: 0x2E55,
	0x2E57: 0x2E58, 0x2E58: 0x2E57, 0x2E59: 0x2E5A, 0x2E5A: 0x2E59,
	0x2E5B: 0x2E5C, 0x2E5C: 0x2E5B, 0x3008: 0x3009, 0x3009: 0x3008,
	0x300A: 0x300B, 0x300B: 0x300A, 0x300C: 0x300D, 0x300D: 0x300C,
	0x300E: 0x300F, 0x300F: 0x300E, 0x3010: 0x3011, 0x3011: 0x3010,
	0x3014: 0x3015, 0x3015: 0x3014, 0x3016: 0x3017, 0x3017: 0x3016,
	0x3018: 0x3019, 0x3019: 0x3018, 0x301A: 0x301B, 0x301B: 0x301A,
	0xFE59: 0xFE5A, 0xFE5A: 0xFE59, 0xFE5B: 0xFE5C, 0xFE5C: 0xFE5B,
	0xFE5D: 0xFE5E, 0xFE5E: 0xFE5D, 0xFF08: 0xFF09, 0xFF09: 0xFF08,
	0xFF3B: 0xFF3D, 0xFF3D: 0xFF3B, 0xFF5B: 0xFF5D, 0xFF5D: 0xFF5B,
	0xFF5F: 0xFF60, 0xFF60: 0xFF5F, 0xFF62: 0xFF63, 0xFF63: 0xFF62,
}

type Direction int

const (
	LeftToRight Direction = iota
	RightToLeft
)

// TODO should we check lvl < maxDepth or lvl <= maxDepth

// OPT can we turn the 'if (lvl%2 == 0 && c == bidi.RLE) || (lvl%2 != 0 && c ==
// bidi.LRE) {' pattern into something without branches, by treating bools as
// integers?

// The maximum embedding level, as specified by BD2. We could easily support an
// arbitrary number of stack entries, only limited by available memory, but the
// standard specifies a maximum value "to provide a precise stack limit for
// implementations to guarantee the same results". The implementation would be
// slightly more straightforward without such a limit.
const maxEmbeddingDepth = 125

// The spec says that the max bracket depth is 63, but the reference
// implementation allows for 63 levels, so a max depth of 62.
const maxBracketDepth = 62

type directionalOverride uint8

const (
	neutral directionalOverride = iota
	rtlOverride
	ltrOverride
)

type directionalStatus struct {
	// the rune's index
	index          int
	embeddingLevel int8
	// OPT directionalOverride needs 2 bits, directionalIsolateStatus needs 1 bit.
	//
	// OPT are directional override and isolate status mutually exclusive? then
	// we only need 2 bits for both, not 3, as the override currently only has 3
	// out of 4 possible values.
	//
	// OPT note that we'll have at most 256 instances of directionalStatus, of
	// which only maxDepth will ever be accessed. While there is some benefit to
	// keeping this small, it's not crucial.
	directionalOverrideStatus directionalOverride
	// TODO rename to isIsolate
	directionalIsolateStatus bool
}

type embeddingStack struct {
	// OPT make it so stack operations don't need bounds checks. probably make n
	// uint8 and resize values to 256 elements.

	n      int
	values [maxEmbeddingDepth + 2]directionalStatus
}

func (s *embeddingStack) length() int {
	return s.n
}

func (s *embeddingStack) pop() directionalStatus {
	s.n--
	return s.values[s.n]
}

func (s *embeddingStack) peek() directionalStatus {
	return s.values[s.n-1]
}

func (s *embeddingStack) push(status directionalStatus) {
	if s.n >= len(s.values) {
		panic("internal error: stack is full")
	}
	s.values[s.n] = status
	s.n++
}

type bracketStackEntry struct {
	position  int
	character rune
}

type bracketStack struct {
	// OPT make it so stack operations don't need bounds checks. probably make n
	// uint8 and resize values to 256 elements.

	n      int
	values [maxBracketDepth + 1]bracketStackEntry
}

func (s *bracketStack) at(index int) bracketStackEntry {
	return s.values[index]
}

func (s *bracketStack) trim(n int) {
	s.n = n
}

func (s *bracketStack) length() int {
	return s.n
}

func (s *bracketStack) pop() bracketStackEntry {
	s.n--
	return s.values[s.n]
}

func (s *bracketStack) peek() bracketStackEntry {
	return s.values[s.n-1]
}

func (s *bracketStack) push(entry bracketStackEntry) {
	if s.n >= len(s.values) {
		panic("internal error: stack is full")
	}
	s.values[s.n] = entry
	s.n++
}

type bitset []uint64

func newBitset(n int) bitset {
	return make([]uint64, (n+63)/64)
}

func (bs bitset) get(idx int) bool {
	return (bs[idx/64]>>(idx%64))&1 != 0
}

func (bs bitset) getAsClass(idx int) bidi.Class {
	if bs.get(idx) {
		return bidi.R
	} else {
		return bidi.L
	}
}

func (bs bitset) set(idx int) {
	bs[idx/64] |= 1 << (idx % 64)
}

type Paragraph struct {
	Classes []bidi.Class
	Levels  []int8
}

type Instance struct {
	// Do not remove BNs and explicit formatting characters from text runs. This
	// modifies the algorithm according to the notes from "Retaining BNs and
	// Explicit Formatting Characters". Note that this can alter the
	// interpretation of the rest of the text in some edge cases and isn't
	// conformant.
	//
	// Retaining these characters is primarily useful for displaying graphical
	// representations for them, as this requires them to be ordered correctly.
	RetainFormattingCharacters bool

	ParagraphDirection Direction
}

func printTrace(when string, runes []rune, classes []bidi.Class, levels []int8, seqs []isolatingRunSequence, sos, eos bitset) {
	if !trace {
		return
	}
	fmt.Println("Trace:", when)

	fmt.Print("Position:\t")
	for i := range runes {
		fmt.Printf(" %6d", i)
	}
	fmt.Println()

	fmt.Print("Text:\t\t")
	for _, r := range runes {
		fmt.Printf(" %06X", r)
	}
	fmt.Println()

	fmt.Print("Bidi_Class:\t")
	for _, c := range classes {
		switch c {
		case bidi.L:
			fmt.Printf(" %6s", "L")
		case bidi.R:
			fmt.Printf(" %6s", "R")
		case bidi.EN:
			fmt.Printf(" %6s", "EN")
		case bidi.ES:
			fmt.Printf(" %6s", "ES")
		case bidi.ET:
			fmt.Printf(" %6s", "ET")
		case bidi.AN:
			fmt.Printf(" %6s", "AN")
		case bidi.CS:
			fmt.Printf(" %6s", "CS")
		case bidi.B:
			fmt.Printf(" %6s", "B")
		case bidi.S:
			fmt.Printf(" %6s", "S")
		case bidi.WS:
			fmt.Printf(" %6s", "WS")
		case bidi.ON:
			fmt.Printf(" %6s", "ON")
		case bidi.BN:
			fmt.Printf(" %6s", "BN")
		case bidi.NSM:
			fmt.Printf(" %6s", "NSM")
		case bidi.AL:
			fmt.Printf(" %6s", "AL")
		case bidi.LRO:
			fmt.Printf(" %6s", "LRO")
		case bidi.RLO:
			fmt.Printf(" %6s", "RLO")
		case bidi.LRE:
			fmt.Printf(" %6s", "LRE")
		case bidi.RLE:
			fmt.Printf(" %6s", "RLE")
		case bidi.PDF:
			fmt.Printf(" %6s", "PDF")
		case bidi.LRI:
			fmt.Printf(" %6s", "LRI")
		case bidi.RLI:
			fmt.Printf(" %6s", "RLI")
		case bidi.FSI:
			fmt.Printf(" %6s", "FSI")
		case bidi.PDI:
			fmt.Printf(" %6s", "PDI")
		default:
			fmt.Print(c)
		}
	}
	fmt.Println()

	fmt.Print("Levels:\t\t")
	for _, lvl := range levels {
		if lvl == -1 {
			fmt.Print("      x")
		} else {
			fmt.Printf(" %6d", lvl)
		}
	}
	fmt.Println()

	for seqIdx, seq := range seqs {
		fmt.Print("Seqs:\t\t")
		cursor := 0

		for _, run := range seq.runs {
			for i := cursor; i < run.start; i++ {
				fmt.Print("       ")
			}
			cursor = run.start
			fmt.Print(" ")
			if sos.get(seqIdx) {
				fmt.Print("<R..")
			} else {
				fmt.Print("<L..")
			}
			for range max(0, (run.end-1-cursor)*7) {
				fmt.Print(".")
			}
			if eos.get(seqIdx) {
				fmt.Print("R>")
			} else {
				fmt.Print("L>")
			}
		}

		fmt.Println()
	}

	fmt.Println()
}

func (th *Instance) Process(text []rune) Paragraph {
	if len(text) == 0 {
		return Paragraph{}
	}

	// TODO implement P1 and split text into paragraphs. For now we assume that
	// text contains a single paragraph only.

	// TODO implement P2 and P3 to allow paragraphDirection to be omited

	var paragraphEmbeddingLevel int8
	// A higher-level protocol may specify the paragraph level, as per HL1.
	switch th.ParagraphDirection {
	case LeftToRight:
		paragraphEmbeddingLevel = 0
	case RightToLeft:
		paragraphEmbeddingLevel = 1
	default:
		panic(fmt.Sprintf("invalid Direction %v", th.ParagraphDirection))
	}

	// OPT instead of having one entry per rune, track runs of runes and use
	// binary search for lookups. for non-adversarial inputs this will massively
	// reduce memory usage, using memory proportional to the number of class and
	// level changes instead of to the length of the text.
	//
	// TODO don't depend on x/text/unicode/bidi
	// OPT bidi.Class is type int, but 8 bits would suffice
	// OPT can we make do without an explicit runeClasses slice?
	// OPT allow reusing slices for multiple calls of Entry
	runeClasses := make([]bidi.Class, len(text))
	//
	// We could parallelize this work, but it would only pay off for
	// absurdly long paragraphs. Testing on a 3950x, to benefit from two
	// goroutines and to offset the cost of synchronization would require more
	// than 5000 characters. That is more characters than an average page of
	// text. Since that will almost certainly involve multiple paragraphs, the
	// user can parallelize on a per-paragraph level instead.
	for i, r := range text {
		props, _ := bidi.LookupRune(r)
		runeClasses[i] = props.Class()
	}

	embeddingLevels := make([]int8, len(text))

	printTrace("Initial", text, runeClasses, embeddingLevels, nil, nil, nil)

	// isolatePDIs maps from valid LRIs, RLIs, and FSIs to their matching PDIs,
	// if any.
	isolatePDIs := make(map[int]int)

	// X1

	var directionalStatusStack embeddingStack
	directionalStatusStack.push(directionalStatus{
		index:                     -1,
		embeddingLevel:            paragraphEmbeddingLevel,
		directionalOverrideStatus: neutral,
		directionalIsolateStatus:  false,
	})

	// Number of isolate initiators that exceeded the depth limit and that we
	// haven't encountered a matching PDI for yet. Used to correctly match PDIs
	// with initiators when we can't use the stack. Also used (as a boolean) to
	// ignore PDFs within overflowing isolates.
	var overflowIsolateCount int

	// Number of embedding initiators that exceeded the depth limit and that we
	// haven't encountered a matching PDF or a parent's PDI for yet. Used to
	// correctly match PDFs with initiators when we can't use the stack.
	//
	// This count does not include embedding initiators encountered while in an
	// overflowing isolate. Such initiators are terminated when the overflow
	// isolate count reaches zero.
	var overflowEmbeddingCount int

	// Number of isolate initiators on the stack that we haven't encountered a
	// matching PDI for yet. It is the same as the number of stack entries with
	// directionalIsolateStatus == true.
	//
	// Used to efficiently check if a PDI has a matching isolate initiator.
	var validIsolateCount int

	// A bit set marking runes that are valid PDIs with matching initiators.
	validPDIs := newBitset(len(text))

	for i, c := range runeClasses {
		if th.RetainFormattingCharacters {
			switch c {
			case bidi.RLE, bidi.LRE, bidi.RLO, bidi.LRO:
				// Applying the effect of "Retaining BNs and Explicit Formatting
				// Characters" on X2-X5
				embeddingLevels[i] = directionalStatusStack.peek().embeddingLevel
			}
		}

		switch c {
		case bidi.RLE, bidi.LRE: // X2 and X3
			lvl := directionalStatusStack.peek().embeddingLevel + 1
			if (lvl%2 != 1 && c == bidi.RLE) || (lvl%2 != 0 && c == bidi.LRE) {
				lvl++
			}
			if lvl <= maxEmbeddingDepth && overflowIsolateCount == 0 && overflowEmbeddingCount == 0 {
				directionalStatusStack.push(directionalStatus{
					index:                     i,
					embeddingLevel:            lvl,
					directionalOverrideStatus: neutral,
					directionalIsolateStatus:  false,
				})
			} else {
				if overflowIsolateCount == 0 {
					overflowEmbeddingCount++
				}
			}
		case bidi.RLO, bidi.LRO: // X4, X5
			lvl := directionalStatusStack.peek().embeddingLevel + 1
			if (lvl%2 == 0 && c == bidi.RLO) || (lvl%2 != 0 && c == bidi.LRO) {
				lvl++
			}
			if lvl <= maxEmbeddingDepth && overflowIsolateCount == 0 && overflowEmbeddingCount == 0 {
				var over directionalOverride
				switch c {
				case bidi.RLO:
					over = rtlOverride
				case bidi.LRO:
					over = ltrOverride
				}
				directionalStatusStack.push(directionalStatus{
					index:                     i,
					embeddingLevel:            lvl,
					directionalOverrideStatus: over,
					directionalIsolateStatus:  false,
				})
			} else {
				if overflowIsolateCount == 0 {
					overflowEmbeddingCount++
				}
			}
		case bidi.RLI, bidi.LRI, bidi.FSI: // X5a, X5b, X5c
			if c == bidi.FSI {
				// FIXME OPT For FSI, we need to find the first strong character
				// between the FSI and its matching PDI, skipping over nested
				// isolates. Doing this naively, by scanning forward every time
				// we see an FSI, means that malicious input of the kind "FSI
				// FSI FSI ..." results in quadratic behavior.

				stack := 1
			fsiLoop:
				for j := i + 1; j < len(runeClasses); j++ {
					switch runeClasses[j] {
					case bidi.L:
						if stack == 1 {
							c = bidi.LRI
							break fsiLoop
						}
					case bidi.R, bidi.AL:
						if stack == 1 {
							c = bidi.RLI
							break fsiLoop
						}
					case bidi.FSI, bidi.LRI, bidi.RLI:
						stack++
					case bidi.PDI:
						stack--
						if stack == 0 {
							break fsiLoop
						}
					}
				}

				if c == bidi.FSI {
					// We didn't find any strong character in the isolate,
					// default to left-to-right.
					c = bidi.LRI
				}
			}

			status := directionalStatusStack.peek()
			embeddingLevels[i] = status.embeddingLevel
			switch status.directionalOverrideStatus {
			case neutral:
				// nothing to do
			case ltrOverride:
				runeClasses[i] = bidi.L
			case rtlOverride:
				runeClasses[i] = bidi.R
			}
			lvl := status.embeddingLevel + 1
			if (lvl%2 == 0 && c == bidi.RLI) || (lvl%2 != 0 && c == bidi.LRI) {
				lvl++
			}
			if lvl <= maxEmbeddingDepth && overflowIsolateCount == 0 && overflowEmbeddingCount == 0 {
				validIsolateCount++
				directionalStatusStack.push(directionalStatus{
					index:                     i,
					embeddingLevel:            lvl,
					directionalOverrideStatus: neutral,
					directionalIsolateStatus:  true,
				})
			} else {
				overflowIsolateCount++
			}

		case bidi.PDI: // X6a
			if overflowIsolateCount > 0 {
				// PDI matches an overflowing isolate initiator.
				overflowIsolateCount--
			} else if validIsolateCount == 0 {
				// PDI doesn't match any isolate initiator, do nothing.
			} else {
				// PDI matches a valid isolate initiator.

				// Terminate overflowing embedding initiators within this
				// isolate's scope that are missing PDFs.
				overflowEmbeddingCount = 0

				// Terminate embedding initiators within this isolate's scope
				// that are missing PDFs.
				for !directionalStatusStack.peek().directionalIsolateStatus {
					directionalStatusStack.pop()
				}

				// Terminate isolate.
				s := directionalStatusStack.pop()
				validIsolateCount--

				// Mark valid PDI.
				validPDIs.set(i)

				isolatePDIs[s.index] = i
			}

			entry := directionalStatusStack.peek()
			embeddingLevels[i] = entry.embeddingLevel
			switch entry.directionalOverrideStatus {
			case rtlOverride:
				runeClasses[i] = bidi.R
			case ltrOverride:
				runeClasses[i] = bidi.L
			case neutral:
				// nothing to do
			}

		case bidi.PDF: // X7
			if overflowIsolateCount > 0 {
				// Do nothing. The PDF either matches an overflow embedding
				// initiator or it doesn't match; either case is handled
				// implicitly when the isolate terminates.
			} else if overflowEmbeddingCount > 0 {
				// PDF matches an overflowing embedding initiator that is not
				// within an overflowing isolate initiator. Terminate the
				// embedding.
				overflowEmbeddingCount--
			} else if !directionalStatusStack.peek().directionalIsolateStatus && directionalStatusStack.length() >= 2 {
				// PDF matches and terminates a valid embedding initiator.
				directionalStatusStack.pop()
			} else {
				// Do nothing. The PDF does not match any embedding initiator.
			}

			if th.RetainFormattingCharacters {
				// Applying the effect of "Retaining BNs and Explicit Formatting
				// Characters" on X7, which means we have to unconditionally set the
				// embedding levels of PDFs.
				embeddingLevels[i] = directionalStatusStack.peek().embeddingLevel
			}

		case bidi.BN: // X6
			if th.RetainFormattingCharacters {
				// Applying the effect of "Retaining BNs and Explicit Formatting
				// Characters" on X6, which means we have to update the embedding
				// levels of BNs, without changing their character classes.
				status := directionalStatusStack.peek()
				embeddingLevels[i] = status.embeddingLevel
			}

		case bidi.B:
			// TODO if we were to support multiple paragraphs in one call, we'd
			// have to do more work here, such as clearing the stack.
			embeddingLevels[i] = paragraphEmbeddingLevel

		case bidi.L, bidi.R, bidi.EN, bidi.ES, bidi.ET, bidi.AN, bidi.CS,
			bidi.S, bidi.WS, bidi.ON, bidi.NSM, bidi.AL, bidi.Control: // X6

			// This is the default branch. We list all possible values
			// explicitly so that this compiles to a jump table.

			status := directionalStatusStack.peek()
			embeddingLevels[i] = status.embeddingLevel
			switch status.directionalOverrideStatus {
			case ltrOverride:
				runeClasses[i] = bidi.L
			case rtlOverride:
				runeClasses[i] = bidi.R
			case neutral:
				// nothing to do
			}
		}
	}

	printTrace("X1-X8", text, runeClasses, embeddingLevels, nil, nil, nil)

	// X9
	if th.RetainFormattingCharacters {
		for i, c := range runeClasses {
			// This turns RLE, LRE, RLO, LRO, and PDF into BN
			// OPT use >= and <= instead of a switch
			switch c {
			case bidi.RLE, bidi.LRE, bidi.RLO, bidi.LRO, bidi.PDF:
				runeClasses[i] = bidi.BN
			}
		}
	} else {
		for i, c := range runeClasses {
			// This marks RLE, LRE, RLO, LRO, and PDF as deleted.
			// OPT use >= and <= instead of a switch
			switch c {
			case bidi.RLE, bidi.LRE, bidi.RLO, bidi.LRO, bidi.PDF, bidi.BN:
				embeddingLevels[i] = -1
			}
		}
	}

	printTrace("X9", text, runeClasses, embeddingLevels, nil, nil, nil)

	// X10, BD13; compute isolating run sequences
	levelRuns := func(yield func(start, end int) bool) {
		start := 0
		for ; start < len(embeddingLevels) && embeddingLevels[start] == -1; start++ {
		}
		if start == len(embeddingLevels) {
			return
		}
		curLevel := embeddingLevels[start]
		for i, lvl := range embeddingLevels {
			if lvl == -1 {
				continue
			}
			if lvl != curLevel {
				if !yield(start, i) {
					return
				}
				start = i
				curLevel = lvl
			}
		}
		// Cut off trailing deleted characters
		end := len(embeddingLevels)
		for ; end >= 1 && embeddingLevels[end-1] == -1; end-- {
		}
		if end > start {
			yield(start, end)
		}
	}

	// OPT check if isolatingRunSequences can be an iterator instead
	var isolatingRunSequences []isolatingRunSequence
	for start, end := range levelRuns {
		if validPDIs.get(start) {
			continue
		}

		var seq isolatingRunSequence
		seq.runs = append(seq.runs, levelRun{start: start, end: end})

		for {
			// XXX because we retain explicit formatting characters, we have to
			// skip over BNs at the end when looking for isolate initiators.
			last := seq.runs[len(seq.runs)-1].end - 1
			lastClass := runeClasses[last]
			lastLevel := embeddingLevels[last]
			// While the level run currently last in the sequence ends with an
			// isolate initiator...
			//
			if lastClass < bidi.LRI || lastClass > bidi.FSI {
				break
			}
			// ... that has a matching PDI
			pdi, ok := isolatePDIs[last]
			if !ok {
				break
			}
			// ... that must be the first character of its level run
			if pdi == last+1 {
				// An RLI/LRI and its matching PDI have the same embedding
				// level, which is the one before the level is being raised.
				// Thus: PDI is at the start of a level run if there are any
				// characters between the initiator and the PDI. Otherwise, it's
				// in the same run as the initiator and that run's end.
				break
			}

			end := pdi
			for ; end < len(embeddingLevels) && embeddingLevels[end] == lastLevel; end++ {
			}
			// As established previously, the PDI is at the beginning of its
			// containing run.
			seq.runs = append(seq.runs, levelRun{start: pdi, end: end})
		}

		seq.analyze(runeClasses)
		isolatingRunSequences = append(isolatingRunSequences, seq)
	}

	// start-of-sequence and end-of-sequence types. false means L and true means
	// R.
	sos := newBitset(len(isolatingRunSequences))
	eos := newBitset(len(isolatingRunSequences))

	for i, seq := range isolatingRunSequences {
		// "In rule X10, when determining the sos and eos for an isolating run
		// sequence, skip over any BNs when looking for the character preceding
		// the isolating run sequence's first character and following its last
		// character. Do the same when determining if the last character of the
		// sequence is an isolate initiator."
		//
		// But can "the isolating run sequence's first character" be a BN or do
		// we have to skip forward to the first non-BN character?

		// Determine start-of-sequence type

		// Find the first character before the start of the sequence that isn't
		// BN.
		prev := seq.runs[0].start - 1
		for ; prev >= 0 && (embeddingLevels[prev] == -1 || runeClasses[prev] == bidi.BN); prev-- {
		}
		var prevLevel int8
		if prev >= 0 {
			prevLevel = embeddingLevels[prev]
		} else {
			// There is no previous character, use the paragraph level instead.
			prevLevel = paragraphEmbeddingLevel
		}

		thisStart := seq.runs[0].start
		for ; thisStart < seq.runs[len(seq.runs)-1].end && (embeddingLevels[thisStart] == -1 || runeClasses[thisStart] == bidi.BN); thisStart++ {
		}
		var thisLevel int8
		if thisStart < seq.runs[len(seq.runs)-1].end {
			thisLevel = embeddingLevels[thisStart]
		} else {
			thisLevel = prevLevel
		}
		if max(prevLevel, thisLevel)%2 == 1 {
			sos.set(i)
		}

		// Determine end-of-sequence type
		var nextLevel int8
		thisEnd := seq.runs[len(seq.runs)-1].end - 1
		for ; thisEnd >= seq.runs[0].start && (embeddingLevels[thisEnd] == -1 || runeClasses[thisEnd] == bidi.BN); thisEnd-- {
		}
		found := false
		if thisEnd >= seq.runs[0].start {
			if r := runeClasses[thisEnd]; r >= bidi.LRI && r <= bidi.FSI {
				// If the last character of the sequence is an isolate initiator,
				// use the paragraph embedding level.
				nextLevel = paragraphEmbeddingLevel
				found = true
			}
		}
		if !found {
			// Find the first character after the end of the sequence that isn't BN.
			next := seq.runs[len(seq.runs)-1].end
			for ; next < len(runeClasses) && (embeddingLevels[next] == -1 || runeClasses[next] == bidi.BN); next++ {
			}
			if next < len(runeClasses) {
				nextLevel = embeddingLevels[next]
			} else {
				// There is no next character, use the paragraph level instead.
				nextLevel = paragraphEmbeddingLevel
			}
		}
		thisLevel = embeddingLevels[seq.runs[len(seq.runs)-1].end-1]
		if max(nextLevel, thisLevel)%2 == 1 {
			eos.set(i)
		}
	}

	printTrace("X10", text, runeClasses, embeddingLevels, isolatingRunSequences, sos, eos)

	// Resolving weak types
	for seqIdx := range isolatingRunSequences {
		seq := &isolatingRunSequences[seqIdx]
		// W1, W2, W3
		if seq.classes&(NSM|EN|AL) != 0 {
			prevClass := sos.getAsClass(seqIdx)       // W1
			prevStrongClass := sos.getAsClass(seqIdx) // W2
			for i, run := range seqIndices(seq, 0, embeddingLevels) {
				switch c := runeClasses[i]; c {
				case bidi.NSM: // W1
					if prevClass >= bidi.LRI && prevClass <= bidi.PDI {
						run.classes |= ON
						seq.classes |= ON
						runeClasses[i] = bidi.ON
					} else {
						run.classes |= xbidiToClass[prevClass]
						seq.classes |= xbidiToClass[prevClass]
						runeClasses[i] = prevClass
					}
				case bidi.EN: // W2
					if prevStrongClass == bidi.AL {
						run.classes |= AN
						seq.classes |= AN
						runeClasses[i] = bidi.AN
					}
				case bidi.AL:
					run.classes |= R
					seq.classes |= R
					runeClasses[i] = bidi.R // W3
					fallthrough
				case bidi.R, bidi.L:
					prevStrongClass = c // W2
				}
				if runeClasses[i] != bidi.BN { // W1
					// It doesn't matter that this observes changes made by W3
					// and W3, as we only match on prevClass values in the
					// LRI..PDI range. W2 only changes EN to AN, and W3 AL to R.
					prevClass = runeClasses[i]
				}
			}
		}

		// W4
		if seq.classes&(EN|AN|ES|CS) != 0 {
			numClass := ^bidi.Class(0)
			sepIdx := -1
			for i := range seqIndices(seq, 0, embeddingLevels) {
				switch c := runeClasses[i]; c {
				case bidi.EN, bidi.AN:
					if sepIdx != -1 && c == numClass {
						runeClasses[sepIdx] = numClass
					}
					numClass = c
					sepIdx = -1
				case bidi.ES:
					if numClass == bidi.EN && sepIdx == -1 {
						sepIdx = i
					} else {
						numClass = ^bidi.Class(0)
						sepIdx = -1
					}
				case bidi.CS:
					if numClass != ^bidi.Class(0) && sepIdx == -1 {
						sepIdx = i
					} else {
						numClass = ^bidi.Class(0)
						sepIdx = -1
					}
				case bidi.BN:
					// When retaining BNs, scan past them.
				default:
					numClass = ^bidi.Class(0)
					sepIdx = -1
				}
			}
		}

		// OPT combine the two W5 sub-passes

		// W5, BN* ET (BN | ET)* EN
		if seq.classes&(ET|EN) != 0 {
			var state int
			// OPT we don't need to store every index, just contiguous ranges
			var indices []int
			for i := range seqIndices(seq, 0, embeddingLevels) {
				switch state {
				case 0:
					switch runeClasses[i] {
					case bidi.BN:
						indices = append(indices, i)
						state = 1
					case bidi.ET:
						indices = append(indices, i)
						state = 2
					}
				case 1:
					switch runeClasses[i] {
					case bidi.BN:
						indices = append(indices, i)
					case bidi.ET:
						indices = append(indices, i)
						state = 2
					default:
						indices = indices[:0]
						state = 0
					}
				case 2:
					switch runeClasses[i] {
					case bidi.BN, bidi.ET:
						indices = append(indices, i)
					case bidi.EN:
						for _, j := range indices {
							runeClasses[j] = bidi.EN
						}
						indices = indices[:0]
						state = 0
					default:
						indices = indices[:0]
						state = 0
					}
				}
			}
		}

		// W5, EN BN* ET (BN | ET)*
		if seq.classes&(EN|ET) != 0 {
			var state int
			// OPT we don't need to store every index, just contiguous ranges
			var indices []int
			for i := range seqIndices(seq, 0, embeddingLevels) {
				switch state {
				case 0:
					if runeClasses[i] == bidi.EN {
						state = 1
					}
				case 1:
					switch runeClasses[i] {
					case bidi.BN:
						indices = append(indices, i)
					case bidi.ET:
						for _, j := range indices {
							runeClasses[j] = bidi.EN
						}
						indices = indices[:0]
						runeClasses[i] = bidi.EN
						state = 2
					case bidi.EN:
						indices = indices[:0]
					default:
						indices = indices[:0]
						state = 0
					}
				case 2:
					switch runeClasses[i] {
					case bidi.BN, bidi.ET:
						runeClasses[i] = bidi.EN
					default:
						state = 0
					}
				}
			}
		}

		// W6
		if seq.classes&(ET|ES|CS) != 0 {
			var state int
			// OPT we don't need to store every index, just contiguous ranges
			var indices []struct {
				idx int
				run *levelRun
			}
			for i, run := range seqIndices(seq, 0, embeddingLevels) {
				switch state {
				case 0:
					switch runeClasses[i] {
					case bidi.BN:
						// When retaining BNs, change those that are adjacent to
						// ET, ES, or CS.
						indices = append(indices, struct {
							idx int
							run *levelRun
						}{i, run})
					case bidi.ET, bidi.ES, bidi.CS:
						for _, j := range indices {
							j.run.classes |= ON
							runeClasses[j.idx] = bidi.ON
						}
						run.classes |= ON
						seq.classes |= ON
						runeClasses[i] = bidi.ON
						state = 1
					default:
						indices = indices[:0]
					}
				case 1:
					switch runeClasses[i] {
					case bidi.BN, bidi.ET, bidi.ES, bidi.CS:
						seq.classes |= ON
						run.classes |= ON
						runeClasses[i] = bidi.ON
					default:
						state = 0
					}
				}
			}
		}

		// W7
		if seq.classes&EN != 0 {
			prevStrongClass := sos.getAsClass(seqIdx)
			for i, run := range seqIndices(seq, 0, embeddingLevels) {
				switch c := runeClasses[i]; c {
				case bidi.R, bidi.L:
					prevStrongClass = c
				case bidi.EN:
					if prevStrongClass == bidi.L {
						run.classes |= L
						seq.classes |= L
						runeClasses[i] = bidi.L
					}
				}
			}
		}
	}

	printTrace("W1-W7", text, runeClasses, embeddingLevels, nil, nil, nil)

	// Resolving neutral and isolate formatting types

	type bracketPair struct {
		// the positions of the opening and closing brackets, in absolute text
		// coordinates.
		open  int
		close int
	}

	// This implements BD16 for finding bracket pairs in isolating run
	// sequences.
	//
	// TODO see if bracketPairs can be an iterator instead
	bracketPairs := func(seq *isolatingRunSequence) ([]bracketPair, bool) {
		if seq.classes&ON == 0 {
			return nil, true
		}

		var stack bracketStack
		var brackets []bracketPair
		for j := range seqIndicesFilter(seq, 0, embeddingLevels, ON) {
			// As per BD14 and BD15, paired brackets must have the ON character
			// class, using the character classes after previous rules have been
			// applied.
			if runeClasses[j] != bidi.ON {
				continue
			}

			r := text[j]
			props, _ := bidi.LookupRune(r)
			if props.IsBracket() {
				if props.IsOpeningBracket() {
					if stack.length() <= maxBracketDepth {
						stack.push(bracketStackEntry{
							position:  j,
							character: bracketMirrors[r],
						})
					} else {
						return nil, false
					}
				} else {
					for n := stack.length() - 1; n >= 0; n-- {
						top := stack.at(n)
						r_ := r
						if r == 0x3009 {
							r_ = 0x232A
						}
						top_ := top.character
						if top_ == 0x3009 {
							top_ = 0x232A
						}
						if r_ == top_ {
							brackets = append(brackets, bracketPair{
								open:  top.position,
								close: j,
							})
							stack.trim(n)
							break
						}
					}
				}
			}
		}

		slices.SortFunc(brackets, func(a, b bracketPair) int {
			return cmp.Compare(a.open, b.open)
		})
		return brackets, true
	}

	for seqIdx := range isolatingRunSequences {
		seq := &isolatingRunSequences[seqIdx]
		// Storing bracket indices instead of using a bitmap is only marginally
		// faster for text with few brackets, but significantly slower for text
		// with a lot of brackets.
		changedBrackets := newBitset(len(text))

		seqDirection := bidi.L
		if embeddingLevels[seq.runs[0].start]%2 != 0 {
			seqDirection = bidi.R
		}

		// N0
		//
		// Note that N0-N2 do not update the class bitmaps of sequences and
		// runs.
		{
			pairs, ok := bracketPairs(seq)
			if ok {
				for _, pair := range pairs {
					var foundStrong bool
					var foundMatching bool

					// OPT seqIndices scans the sequence from the beginning to find the
					// offset. Since we call this for each bracket pair, it means we're
					// needlessly inspecting prefixes of the sequence over and over,
					// when we know that the new offset can't be less than the old
					// offset. Shouldn't matter much for normal text because sequences
					// won't be that long, but can be abused by malicious inputs.
					for j := range seqIndices(seq, pair.open, embeddingLevels) {
						debug.Assert(j >= pair.open)
						if j == pair.open {
							continue
						}
						debug.Assert(j <= pair.close)
						if j == pair.close {
							break
						}

						rc := runeClasses[j]
						switch rc {
						case bidi.EN, bidi.AN:
							rc = bidi.R
							fallthrough
						case bidi.R, bidi.L:
							foundStrong = true
						}
						if rc == seqDirection {
							// If any strong type (either L or R) matching the
							// embedding direction is found, set the type for both
							// brackets in the pair to match the embedding
							// direction.
							runeClasses[pair.open] = seqDirection
							runeClasses[pair.close] = seqDirection
							changedBrackets.set(pair.open)
							changedBrackets.set(pair.close)
							foundMatching = true
							break
						}
					}

					if !foundMatching && foundStrong {
						var found bool
						// "Otherwise, if there is a strong type it must be opposite
						// the embedding direction. Therefore, test for an
						// established context with a preceding strong type by
						// checking backwards before the opening paired bracket
						// until the first strong type (L or R) is found, using the
						// value of sos if there is none."

						// OPT here we scan backwards to find a strong type. Because
						// we're processing pairs in the forward direction, we only
						// have to scan as far back as the previous pair, possibly
						// reusing the previous pair's result.
					revLoop:
						for k := range seqIndicesReverse(seq, pair.open, embeddingLevels) {
							rck := runeClasses[k]
							switch rck {
							case bidi.EN, bidi.AN:
								rck = bidi.R
							}

							switch rck {
							case bidi.L, bidi.R:
								// If the preceding strong type also doesn't match
								// the embedidng direction, we use the preceding
								// strong type. If it does match the embedding
								// direction, we use the embedding direction. In
								// either case that means we use the preceding
								// strong type.
								runeClasses[pair.open] = rck
								runeClasses[pair.close] = rck
								changedBrackets.set(pair.open)
								changedBrackets.set(pair.close)
								found = true
								break revLoop
							default:
								// Not a strong type, keep looking.
							}
						}

						if !found {
							// We didn't find a preceding strong type, so use use the
							// sos. The sos is either L or R, which either matches
							// seqDirection or it doesn't. In either case, we end up
							// setting the brackets to the sos.
							sosk := sos.getAsClass(seqIdx)
							runeClasses[pair.open] = sosk
							runeClasses[pair.close] = sosk
							changedBrackets.set(pair.open)
							changedBrackets.set(pair.close)
						}
					}
				}

				lookupClass := func(r rune) bidi.Class {
					props, _ := bidi.LookupRune(r)
					return props.Class()
				}
				for n, b := range changedBrackets {
					if b == 0 {
						continue
					}
					var afterBracket bool
					var bracketClass bidi.Class
					for j := range seqIndices(seq, n*64, embeddingLevels) {
						if changedBrackets.get(j) {
							bracketClass = runeClasses[j]
							afterBracket = true
							continue
						}
						if afterBracket {
							if lookupClass(text[j]) == bidi.NSM {
								// Note that we check the rune's original class, before we
								// applied W1.

								// A sequence of NSM after a paired bracket that changed to L or
								// R under N0 changes to match the bracket's type.

								runeClasses[j] = bracketClass
							} else if runeClasses[j] != bidi.BN {
								afterBracket = false
								if j > (n+1)*64 {
									// We've looked at all possible parens in this byte
									break
								}
							}
						} else if j > (n+1)*64 {
							// We've looked at all possible parens in this byte
							break
						}
					}
				}
			}
		}

		// N1
		//
		// We don't check the sequence's classes because NI includes white space
		// and numbers, which virtually all text has.
		//
		// Note that N0-N2 do not update the class bitmaps of sequences and
		// runs.
		{
			start := sos.getAsClass(seqIdx)
			// OPT we don't need to store every index, just contiguous ranges
			var nis []int
			for j := range seqIndices(seq, 0, embeddingLevels) {
				switch runeClasses[j] {
				case bidi.L:
					if start == bidi.L {
						for _, k := range nis {
							runeClasses[k] = bidi.L
						}
					}

					start = bidi.L
					nis = nis[:0]
				case bidi.R, bidi.AN, bidi.EN:
					if start == bidi.R {
						for _, k := range nis {
							runeClasses[k] = bidi.R
						}
					}

					start = bidi.R
					nis = nis[:0]
				case bidi.B, bidi.BN, bidi.S, bidi.WS, bidi.ON, bidi.FSI, bidi.LRI, bidi.RLI, bidi.PDI: // NI
					// OPT with some clever sorting of the constants we might be
					// able to turn this into a >= && <= check instead
					if start == bidi.L || start == bidi.R {
						nis = append(nis, j)
					}
				default:
					start = ^bidi.Class(0)
					nis = nis[:0]
				}
			}

			if class := eos.getAsClass(seqIdx); class == start {
				for _, k := range nis {
					runeClasses[k] = class
				}
			}
		}

		// N2
		//
		// We don't check the sequence's classes because NI includes white space
		// and numbers, which virtually all text has.
		//
		// Note that N0-N2 do not update the class bitmaps of sequences and
		// runs.
		{
			// OPT store ranges not indices
			var indices []int
			var afterNeutral bool
			for j := range seqIndices(seq, 0, embeddingLevels) {
				switch runeClasses[j] {
				case bidi.BN:
					// BNs adjoining neutrals are treated like those neutrals
					if afterNeutral {
						runeClasses[j] = seqDirection
					} else {
						indices = append(indices, j)
					}
				// OPT with some clever sorting of the constants we might be
				// able to turn this into a >= && <= check instead
				// case bidi.B, bidi.S, bidi.WS, bidi.ON:
				case bidi.B, bidi.S, bidi.WS, bidi.ON:
					afterNeutral = true
					for _, k := range indices {
						runeClasses[k] = seqDirection
					}
					indices = indices[:0]
					fallthrough
				case bidi.FSI, bidi.LRI, bidi.RLI, bidi.PDI: // NI
					// TODO the spec says to change the BNs that adjoin "neutrals",
					// but it's not clear if neutrals refers to all of NI or only B,
					// S, WS, and ON
					runeClasses[j] = seqDirection
				default:
					indices = indices[:0]
					afterNeutral = false
				}
			}
		}
	}

	printTrace("N0-N2", text, runeClasses, embeddingLevels, nil, nil, nil)

	for j, c := range runeClasses {
		if embeddingLevels[j] < 0 {
			continue
		}

		if embeddingLevels[j]%2 == 0 {
			// I1
			switch c {
			case bidi.R:
				embeddingLevels[j] += 1
			case bidi.AN, bidi.EN:
				embeddingLevels[j] += 2
			}
		} else {
			// I2
			switch c {
			case bidi.L, bidi.EN, bidi.AN:
				embeddingLevels[j] += 1
			}
		}
	}

	printTrace("I0-I2", text, runeClasses, embeddingLevels, nil, nil, nil)

	return Paragraph{
		Classes: runeClasses,
		Levels:  embeddingLevels,
	}
}

type classBitmap uint32

const (
	L classBitmap = 1 << iota
	R
	EN
	ES
	ET
	AN
	CS
	B
	S
	WS
	ON
	BN
	NSM
	AL
	LRO
	RLO
	LRE
	RLE
	PDF
	LRI
	RLI
	FSI
	PDI

	All = ^uint32(0)
)

var xbidiToClass = [...]classBitmap{
	bidi.L:   L,
	bidi.R:   R,
	bidi.EN:  EN,
	bidi.ES:  ES,
	bidi.ET:  ET,
	bidi.AN:  AN,
	bidi.CS:  CS,
	bidi.B:   B,
	bidi.S:   S,
	bidi.WS:  WS,
	bidi.ON:  ON,
	bidi.BN:  BN,
	bidi.NSM: NSM,
	bidi.AL:  AL,
	bidi.LRO: LRO,
	bidi.RLO: RLO,
	bidi.LRE: LRE,
	bidi.RLE: RLE,
	bidi.PDF: PDF,
	bidi.LRI: LRI,
	bidi.RLI: RLI,
	bidi.FSI: FSI,
	bidi.PDI: PDI,
}

type levelRun struct {
	start, end int
	classes    classBitmap
}

func (run *levelRun) analyze(classes []bidi.Class) {
	run.classes = 0
	for i := run.start; i < run.end; i++ {
		run.classes |= xbidiToClass[classes[i]]
	}
}

// TODO turn seqIndices and seqIndicesReverse into methods

func seqIndices(seq *isolatingRunSequence, start int, levels []int8) iter.Seq2[int, *levelRun] {
	return func(yield func(int, *levelRun) bool) {
		for runIdx := range seq.runs {
			run := &seq.runs[runIdx]
			if start >= run.end {
				continue
			}

			runStart := max(start, run.start)
			for i := runStart; i < run.end; i++ {
				if levels[i] == -1 {
					continue
				}
				if !yield(i, run) {
					return
				}
			}
		}
	}
}

func seqIndicesFilter(seq *isolatingRunSequence, start int, levels []int8, classes classBitmap) iter.Seq2[int, *levelRun] {
	return func(yield func(int, *levelRun) bool) {
		for runIdx := range seq.runs {
			run := &seq.runs[runIdx]
			if start >= run.end {
				continue
			}
			if run.classes&classes == 0 {
				continue
			}

			runStart := max(start, run.start)
			for i := runStart; i < run.end; i++ {
				if levels[i] == -1 {
					continue
				}
				if !yield(i, run) {
					return
				}
			}
		}
	}
}

func seqIndicesReverse(seq *isolatingRunSequence, end int, levels []int8) iter.Seq2[int, *levelRun] {
	return func(yield func(int, *levelRun) bool) {
		for runIdx := len(seq.runs) - 1; runIdx >= 0; runIdx-- {
			run := &seq.runs[runIdx]

			if run.start > end {
				continue
			}

			runEnd := min(run.end, end)
			for i := runEnd - 1; i >= run.start; i-- {
				if levels[i] == -1 {
					continue
				}
				if !yield(i, run) {
					return
				}
			}
		}
	}
}

type isolatingRunSequence struct {
	runs    []levelRun
	classes classBitmap
}

func (seq *isolatingRunSequence) analyze(classes []bidi.Class) {
	seq.classes = 0
	for runIdx := range seq.runs {
		run := &seq.runs[runIdx]
		run.analyze(classes)
		seq.classes |= run.classes
	}
}
