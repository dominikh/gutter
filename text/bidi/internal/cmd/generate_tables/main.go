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

// 3 bits to encode bracket pairs, as a two's complement offset to the paired
// bracket. with the exception of 298F and 298E, all opening brackets have a
// smaller value than their closing pair. instead of spending a bit that we
// cannot afford, we special case 298F/298E with a branch when querying whether
// a bracket is opening or closing.

// 5 bits to encode the bidi class (there are 23 different classes.) we could
// use 4 bits and encode LRO, RLO, LRE, RLE, PDF, LRI, RLI, FSI, and PDI using a
// single value, as these codepoints all have a unique low order nibble. But
// we'd rather not branch every time we look up a character.

const (
	L uint8 = iota
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
)

var stringToConst = map[string]uint8{
	"L":   L,
	"R":   R,
	"EN":  EN,
	"ES":  ES,
	"ET":  ET,
	"AN":  AN,
	"CS":  CS,
	"B":   B,
	"S":   S,
	"WS":  WS,
	"ON":  ON,
	"BN":  BN,
	"NSM": NSM,
	"AL":  AL,
	"LRO": LRO,
	"RLO": RLO,
	"LRE": LRE,
	"RLE": RLE,
	"PDF": PDF,
	"LRI": LRI,
	"RLI": RLI,
	"FSI": FSI,
	"PDI": PDI,
}

var missingStringToConst = map[string]uint8{
	"Arabic_Letter":       AL,
	"European_Terminator": ET,
	"Left_To_Right":       L,
	"Right_To_Left":       R,
}

func parseBidiBrackets() []uint8 {
	data := make([]uint8, 0x10FFFF+1)

	f, err := os.Open("BidiBrackets.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		orig := line
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		line, _, _ = strings.Cut(line, "#")
		parts := strings.Split(line, ";")
		if len(parts) != 3 {
			log.Fatalf("couldn't parse %q", orig)
		}
		this, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 16, 32)
		if err != nil {
			log.Fatal(err)
		}
		pair, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 16, 32)
		if err != nil {
			log.Fatal(err)
		}

		delta := pair - this
		if delta > 3 || delta < -4 {
			log.Fatalf("got delta of %d, expected delta ∈ [-4, 3]", delta)
		}
		if delta < 0 {
			if strings.TrimSpace(parts[2]) == "o" {
				if this != 0x298F {
					log.Fatalf("opening bracket %U has pair that is a smaller code point",
						this)
				}
			}
		}
		data[this] = uint8((delta) & 0b111)
	}
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}

	return data
}

func parseDerivedBidiClass() []uint8 {
	data := make([]uint8, 0x10FFFF+1)

	f, err := os.Open("DerivedBidiClass.txt")
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

	brackets := parseBidiBrackets()
	classes := parseDerivedBidiClass()
	data := make([]uint8, len(brackets))
	for i := range data {
		data[i] = brackets[i]<<5 | classes[i]
	}

	seq := ucdtrie.Compress(data, 256)
	if len(seq.Blocks) > 256 {
		log.Fatalf("got %d unique blocks, expected <=256", len(seq.Blocks))
	}

	f, err := os.Create("data.go")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fmt.Fprintln(f, "// Code generated by generate_tables. DO NOT EDIT.")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "package bidi")
	f.Write(seq.Code("data", "index"))

	seq.PrintStats()
}
