// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package bidi_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"honnef.co/go/gutter/text/bidi"

	xbidi "golang.org/x/text/unicode/bidi"
)

func FuzzBidi(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		runes := []rune(data)
		for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			for _, retain := range []bool{false, true} {
				th := bidi.Instance{
					RetainFormattingCharacters: retain,
					ParagraphDirection:         dir,
				}
				th.Process(runes)
			}
		}
	})
}

func BenchmarkCode(b *testing.B) {
	// Test processing source code
	data, err := os.ReadFile("bidi.go")
	if err != nil {
		b.Fatal(err)
	}
	runes := []rune(string(data))
	for range b.N {
		th := bidi.Instance{
			ParagraphDirection: bidi.LeftToRight,
		}
		th.Process(runes)
	}
	b.ReportMetric(float64(len(runes)*b.N)/b.Elapsed().Seconds(), "runes/s")
}

func BenchmarkWikipedia(b *testing.B) {
	// Test with various texts from Wikipedia, in various LTR and RTL languages.
	m, err := filepath.Glob("./testdata/wikipedia/*.txt")
	if err != nil {
		b.Fatal(err)
	}

	for _, path := range m {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		var runeCount int
		var runes [][]rune
		for para := range bytes.SplitSeq(data, []byte("\n")) {
			para = bytes.TrimSpace(para)
			if len(para) == 0 {
				continue
			}
			paraRunes := []rune(string(para))
			runes = append(runes, paraRunes)
			runeCount += len(paraRunes)
		}

		b.Run(filepath.Base(path)+"-ltr", func(b *testing.B) {
			for range b.N {
				for _, para := range runes {
					th := bidi.Instance{
						ParagraphDirection: bidi.LeftToRight,
					}
					th.Process(para)
				}
			}
			b.ReportMetric(float64(runeCount*b.N)/b.Elapsed().Seconds(), "runes/s")
		})
		b.Run(filepath.Base(path)+"-rtl", func(b *testing.B) {
			for range b.N {
				for _, para := range runes {
					th := bidi.Instance{
						ParagraphDirection: bidi.RightToLeft,
					}
					th.Process(para)
				}
			}
			b.ReportMetric(float64(runeCount*b.N)/b.Elapsed().Seconds(), "runes/s")
		})
	}
}

func BenchmarkTrivial(b *testing.B) {
	// This tests one of the most trivial inputs: a long string of strong
	// characters with the same direction.

	runes := make([]rune, 1000)
	for i := range runes {
		runes[i] = 'A'
	}

	b.ResetTimer()
	for range b.N {
		th := bidi.Instance{
			ParagraphDirection: bidi.LeftToRight,
		}
		th.Process(runes)
	}

	b.ReportMetric(float64(len(runes)*b.N)/b.Elapsed().Seconds(), "runes/s")
}

func BenchmarkNSMAfterBracketDense(b *testing.B) {
	// Text densely packed with parentheses and NSMs.

	runes := make([]rune, 1000)
	for i := 0; i < 1000; i += 4 {
		runes[i] = '('
		runes[i+1] = 'x'
		runes[i+2] = ')'
		runes[i+3] = '\u0331'
	}

	b.ResetTimer()
	for range b.N {
		th := bidi.Instance{
			ParagraphDirection: bidi.LeftToRight,
		}
		th.Process(runes)
	}

	b.ReportMetric(float64(len(runes)*b.N)/b.Elapsed().Seconds(), "runes/s")
}

func BenchmarkNSMAfterBracketSparse(b *testing.B) {
	// Text with some parentheses and NSMs.

	runes := make([]rune, 1000)
	for i := range runes {
		runes[i] = 'a'
	}
	for i := range []int{0, 200, 400, 600, 800} {
		runes[i] = '('
		runes[i+1] = 'x'
		runes[i+2] = ')'
		runes[i+3] = '\u0331'
	}

	b.ResetTimer()
	for range b.N {
		th := bidi.Instance{
			ParagraphDirection: bidi.LeftToRight,
		}
		th.Process(runes)
	}

	b.ReportMetric(float64(len(runes)*b.N)/b.Elapsed().Seconds(), "runes/s")
}

func TestBidi(t *testing.T) {
	t.Parallel()

	f, err := os.Open("./testdata/ucd/BidiTest.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	i := 0

	var levels []int8
	for sc.Scan() {
		i++
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			continue
		}
		if line[0] == '@' {
			if strings.HasPrefix(line, "@Levels:") {
				line = line[len("@Levels: "):]
				levels = nil
				for field := range strings.FieldsSeq(line) {
					if field == "x" {
						levels = append(levels, -1)
					} else {
						lvl, err := strconv.ParseInt(field, 10, 8)
						if err != nil {
							t.Fatal(err)
						}
						levels = append(levels, int8(lvl))
					}
				}
			}
			continue
		}
		levels := levels
		t.Run(fmt.Sprintf("line-%d", i), func(t *testing.T) {
			before, after, found := strings.Cut(line, "; ")
			if !found {
				t.Fatalf("couldn't parse line")
			}

			var runes []rune
			for class := range strings.FieldsSeq(before) {
				rune, ok := runesForClasses[class]
				if !ok {
					t.Fatalf("unknown bidi class %s", class)
				}
				runes = append(runes, rune)
			}

			dirs, err := strconv.ParseInt(after, 10, 4)
			if err != nil {
				t.Fatal(err)
			}
			if dirs&1 != 0 {
				// TODO support Auto-LTR
			}
			if dirs&2 != 0 {
				// LTR
				th := bidi.Instance{
					RetainFormattingCharacters: false,
					ParagraphDirection:         bidi.LeftToRight,
				}
				res := th.Process(runes)
				checkRes(t, res, runes, levels, bidi.LeftToRight, false)

				th = bidi.Instance{
					RetainFormattingCharacters: true,
					ParagraphDirection:         bidi.LeftToRight,
				}
				res = th.Process(runes)
				checkRes(t, res, runes, levels, bidi.LeftToRight, true)
			}
			if dirs&4 != 0 {
				// RTL
				th := bidi.Instance{
					RetainFormattingCharacters: false,
					ParagraphDirection:         bidi.RightToLeft,
				}
				res := th.Process(runes)
				checkRes(t, res, runes, levels, bidi.RightToLeft, false)

				// RTL
				th = bidi.Instance{
					RetainFormattingCharacters: true,
					ParagraphDirection:         bidi.RightToLeft,
				}
				res = th.Process(runes)
				checkRes(t, res, runes, levels, bidi.RightToLeft, true)
			}
		})
	}
}

func checkRes(t *testing.T, res bidi.Paragraph, runes []rune, wantLevels []int8, dir bidi.Direction, retain bool) {
	if len(res.Levels) != len(wantLevels) {
		t.Fatalf("got %d levels, expected %d (dir=%v, retain=%t)", len(res.Levels), len(wantLevels), dir, retain)
	}

	if retain {
		// BidiTest assumes that certain characters have been deleted, but we've
		// turned them into BNs and then some of the BNs further turned into other
		// classes. Go over all characters in runes, and delete those that would've
		// been deleted in a standard implementation.
		// -1.
		for i, r := range runes {
			const (
				RLE = 0x202B
				LRE = 0x202A
				RLO = 0x202E
				LRO = 0x202D
				PDF = 0x202C
			)

			props, _ := xbidi.LookupRune(r)
			if props.Class() == xbidi.BN {
				res.Levels[i] = -1
			} else {
				switch r {
				case RLE, LRE, RLO, LRO, PDF:
					res.Levels[i] = -1
				}
			}
		}
	}

	// BidiTest assumes that we've run L1. For the time being we do that here.
	eol := true
	preceeding := false
	var paraLevel int8
	if dir == bidi.RightToLeft {
		paraLevel = 1
	}
	for i := len(res.Levels) - 1; i >= 0; i-- {
		switch runes[i] {
		case runesForClasses["S"], runesForClasses["B"]:
			res.Levels[i] = paraLevel
			preceeding = true
			eol = false
		case runesForClasses["LRE"], runesForClasses["RLE"], runesForClasses["LRO"], runesForClasses["RLO"], runesForClasses["PDF"], runesForClasses["BN"]:
			if retain {
				// XXX implement
			}
		case runesForClasses["FSI"], runesForClasses["LRI"], runesForClasses["RLI"], runesForClasses["PDI"], runesForClasses["WS"]:
			if eol || preceeding {
				res.Levels[i] = paraLevel
			}
		default:
			eol = false
			preceeding = false
		}
	}

	for j := range wantLevels {
		if got, want := res.Levels[j], wantLevels[j]; got != want {
			t.Fatalf("res.Levels[%d] = %d, want %d, (dir=%v, retain=%t)", j, got, want, dir, retain)
		}
	}
}

var runesForClasses = map[string]rune{
	"L":   '\u0061',
	"R":   '\u05d0',
	"EN":  '\u0030',
	"ES":  '\u002B',
	"ET":  '\u0023',
	"AN":  '\u0661',
	"CS":  '\u002E',
	"B":   '\u000A',
	"S":   '\u000B',
	"WS":  '\u0020',
	"ON":  '\u0021',
	"BN":  '\u0000',
	"NSM": '\u0300',
	"AL":  '\u0608',
	"LRO": '\u202D',
	"RLO": '\u202e',
	"LRE": '\u202A',
	"RLE": '\u202B',
	"PDF": '\u202C',
	"LRI": '\u2066',
	"RLI": '\u2067',
	"FSI": '\u2068',
	"PDI": '\u2069',
}

func TestCharacter(t *testing.T) {
	t.Parallel()

	f, err := os.Open("./testdata/ucd/BidiCharacterTest.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	i := 0

	for sc.Scan() {
		i++
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			continue
		}
		t.Run(fmt.Sprintf("line-%d", i), func(t *testing.T) {
			fields := strings.Split(line, ";")
			if len(fields) != 5 {
				t.Fatalf("unrecognized line %q", line)
			}

			var dir bidi.Direction
			switch fields[1] {
			case "0":
				dir = bidi.LeftToRight
			case "1":
				dir = bidi.RightToLeft
			case "2":
				t.Skip("skipping unsupported auto-LTR paragraph direction")
			default:
				t.Fatalf("unknown paragraph direction %q", fields[1])
			}

			cpoints := strings.Fields(fields[0])
			strLevels := strings.Fields(fields[3])
			var levels []int8
			for _, s := range strLevels {
				if s == "x" {
					levels = append(levels, -1)
				} else {
					n, err := strconv.ParseInt(s, 10, 8)
					if err != nil {
						t.Fatal(err)
					}
					levels = append(levels, int8(n))
				}
			}

			if len(cpoints) != len(levels) {
				t.Fatalf("line specifies %d code points but %d resolved levels",
					len(cpoints), len(levels))
			}

			runes := make([]rune, 0, len(cpoints))
			for _, cpoint := range cpoints {
				r, err := strconv.ParseInt(cpoint, 16, 24)
				if err != nil {
					t.Fatalf("couldn't parse code point %q: %s", cpoint, err)
				}
				runes = append(runes, rune(r))
			}

			th := bidi.Instance{
				RetainFormattingCharacters: false,
				ParagraphDirection:         dir,
			}
			res := th.Process(runes)
			checkRes(t, res, runes, levels, dir, false)

			// This test has an LRI LRI PDF PDF sequence. When retaining
			// formatting characters, this introduces an extra run, which
			// changes the way the whitespace after it resolves.
			if line == "05D0 202A 202A 202C 202C 0020 0031 0020 0032;0;0;1 x x x x 1 2 1 2;8 7 6 5 0" && dir == bidi.LeftToRight {
				return
			}
			// This test has an RLI RLI PDF PDF sequence and behaves similarly
			// to the previous skipped test.
			if line == "0061 202B 202B 202C 202C 0020 0031 0020 0032;1;1;2 x x x x 2 2 2 2;0 5 6 7 8" && dir == bidi.RightToLeft {
				return
			}

			th = bidi.Instance{
				RetainFormattingCharacters: true,
				ParagraphDirection:         dir,
			}
			res = th.Process(runes)
			checkRes(t, res, runes, levels, dir, true)
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}
