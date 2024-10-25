// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"honnef.co/go/gutter/internal/ucdtrie"
)

const blockSize = 128

const (
	XX  uint8 = iota // Unknown
	BK               // Mandatory Break
	CR               // Carriage Return
	LF               // Line Feed
	CM               // Combining Mark
	NL               // Next Line
	SG               // Surrogate
	WJ               // Word Joiner
	ZW               // Zero Width Space
	GL               // Non-breaking (“Glue”)
	SP               // Space
	ZWJ              // Zero Width Joiner
	B2               // Break Opportunity Before and After
	BA               // Break After
	BB               // Break Before
	HY               // Hyphen
	CB               // Contingent Break Opportunity
	CL               // Close Punctuation
	CP               // Close Parenthesis
	EX               // Exclamation/ Interrogation
	IN               // Inseparable
	NS               // Nonstarter
	OP               // Open Punctuation
	QU               // Quotation
	IS               // Infix Numeric Separator
	NU               // Numeric
	PO               // Postfix Numeric
	PR               // Prefix Numeric
	SY               // Symbols Allowing Break After
	AI               // Ambiguous (Alphabetic or Ideographic)
	AK               // Aksara
	AL               // Alphabetic
	AP               // Aksara Pre-Base
	AS               // Aksara Start
	CJ               // Conditional Japanese Starter
	EB               // Emoji Base
	EM               // Emoji Modifier
	H2               // Hangul LV Syllable
	H3               // Hangul LVT Syllable
	HL               // Hebrew Letter
	ID               // Ideographic
	JL               // Hangul L Jamo
	JV               // Hangul V Jamo
	JT               // Hangul T Jamo
	RI               // Regional Indicator
	SA               // Complex Context Dependent (South East Asian)
	VF               // Virama Final
	VI               // Virama
)

var stringToConst = map[string]uint8{
	"XX": XX, "BK": BK, "CR": CR, "LF": LF,
	"CM": CM, "NL": NL, "SG": SG, "WJ": WJ,
	"ZW": ZW, "GL": GL, "SP": SP, "ZWJ": ZWJ,
	"B2": B2, "BA": BA, "BB": BB, "HY": HY,
	"CB": CB, "CL": CL, "CP": CP, "EX": EX,
	"IN": IN, "NS": NS, "OP": OP, "QU": QU,
	"IS": IS, "NU": NU, "PO": PO, "PR": PR,
	"SY": SY, "AI": AI, "AK": AK, "AL": AL,
	"AP": AP, "AS": AS, "CJ": CJ, "EB": EB,
	"EM": EM, "H2": H2, "H3": H3, "HL": HL,
	"ID": ID, "JL": JL, "JV": JV, "JT": JT,
	"RI": RI, "SA": SA, "VF": VF, "VI": VI,
}

var missingStringToConst = map[string]uint8{
	"Unknown":        XX,
	"Prefix_Numeric": PR,
	"Ideographic":    ID,
}

func parseDerivedLineBreak() []uint8 {
	data := make([]uint8, 0x10FFFF+1)

	f, err := os.Open("DerivedLineBreak.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		orig := line
		if !strings.HasPrefix(line, "# @missing: ") {
			continue
		}
		line = strings.TrimPrefix(line, "# @missing: ")
		line, _, _ = strings.Cut(line, "#")
		rng, value, ok := strings.Cut(line, ";")
		if !ok {
			log.Fatalf("couldn't parse %q", orig)
		}
		var start, end int64
		left, right, ok := strings.Cut(rng, "..")
		if ok {
			start, err = strconv.ParseInt(left, 16, 32)
			if err != nil {
				log.Fatalf("couldn't parse %q", orig)
			}
			end, err = strconv.ParseInt(right, 16, 32)
			if err != nil {
				log.Fatalf("couldn't parse %q", orig)
			}
		} else {
			start, err = strconv.ParseInt(left, 16, 32)
			if err != nil {
				log.Fatalf("couldn't parse %q", orig)
			}
			end = start
		}

		value = strings.TrimSpace(value)
		if k, ok := missingStringToConst[value]; ok {
			for i := start; i < end+1; i++ {
				data[i] = k
			}
		} else {
			log.Fatalf("couldn't parse %q", orig)
		}
	}
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}

	if _, err := f.Seek(0, 0); err != nil {
		log.Fatal(err)
	}
	s = bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		orig := line
		if strings.HasPrefix(line, "#") {
			continue
		}
		line, _, _ = strings.Cut(line, "#")
		rng, value, ok := strings.Cut(line, ";")
		if !ok {
			log.Fatalf("couldn't parse %q", orig)
		}
		rng = strings.TrimSpace(rng)
		var start, end int64
		left, right, ok := strings.Cut(rng, "..")
		if ok {
			start, err = strconv.ParseInt(left, 16, 32)
			if err != nil {
				log.Fatalf("couldn't parse %q", orig)
			}
			end, err = strconv.ParseInt(right, 16, 32)
			if err != nil {
				log.Fatalf("couldn't parse %q", orig)
			}
		} else {
			start, err = strconv.ParseInt(left, 16, 32)
			if err != nil {
				log.Fatalf("couldn't parse %q", orig)
			}
			end = start
		}

		value = strings.TrimSpace(value)
		if k, ok := stringToConst[value]; ok {
			_ = data[start]
			_ = data[end]
			for i := start; i < end+1; i++ {
				data[i] = k
			}
		} else {
			log.Fatalf("couldn't parse %q", orig)
		}
	}
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}

	return data
}

func main() {
	// Note that this code trusts that UCD files are well-formed and doesn't
	// sanitize inputs.

	data := parseDerivedLineBreak()
	seq := ucdtrie.Compress(data, 256)
	if len(seq.Blocks) > 256 {
		log.Fatalf("got %d unique blocks, expected <=256", len(seq.Blocks))
	}

	f, err := os.Create("data.go")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fmt.Fprintln(f, "// SPDX-FileCopyrightText: none")
	fmt.Fprintln(f, "//")
	fmt.Fprintln(f, "// SPDX-License-Identifier: CC0-1.0")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "// Code generated by generate_tables. DO NOT EDIT.")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "package linebreak")
	f.Write(seq.Code("data", "index"))

	seq.PrintStats()
}
