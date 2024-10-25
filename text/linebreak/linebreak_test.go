// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package linebreak_test

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"honnef.co/go/gutter/text/linebreak"
	"honnef.co/go/safeish"
)

func FuzzProcess(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		ins := linebreak.Instance{}
		ins.Process([]rune(data))
	})
}

func BenchmarkWikipedia(b *testing.B) {
	// Test with various texts from Wikipedia, in various LTR and RTL languages.
	m, err := filepath.Glob("./testdata/wikipedia/*.txt")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range m {
		b.Run(fmt.Sprintf("text=%s", filepath.Base(f)), func(b *testing.B) {
			data, err := os.ReadFile(f)
			if err != nil {
				b.Fatal(err)
			}
			runes := []rune(string(data))

			b.ResetTimer()
			for range b.N {
				ins := linebreak.Instance{}
				ins.Process(runes)
			}
			b.ReportMetric(float64(len(runes)*b.N)/b.Elapsed().Seconds(), "runes/s")
		})
	}
}

func TestWikipedia(t *testing.T) {
	hashes := map[string]string{
		"arabic_ar.txt": "d29d0ff0fe22bba1299e2ae975c359035b96092eb4c57a7cbe87ae4a6010adb8",
		"arabic_en.txt": "e5cd02e308a7eadd53f5b6325d16b5e5cba2d386efda5cc7c6480841f4e53f0a",
		"arabic_he.txt": "6801e28e801766c71b3399f1cfdf8a05ae3e50e27a1b819aa1fe55d65360fec6",
		"arabic_jp.txt": "7b0b4bc2d7f668f7bd105a5f9298b0ca9f7525eb450a00a562c6e597debadefa",
		"arabic_ko.txt": "20bbd515c6b2544f187e3b03044395a497a34c878ee6ca3ee1c17817f84450f8",
		"arabic_zh.txt": "cb7af079972b2b4c129b30b170e50da5467bf2de63ecd3da8bc30c217178cc35",
	}

	m, err := filepath.Glob("./testdata/wikipedia/*.txt")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range m {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		runes := []rune(string(data))

		ins := linebreak.Instance{}
		ret := ins.Process(runes)
		sum := sha256.Sum256(safeish.SliceCast[[]byte](ret))
		got := fmt.Sprintf("%x", sum)
		want := hashes[filepath.Base(f)]
		if got != want {
			t.Errorf("%s: got hash %s, want %s", filepath.Base(f), got, want)
		}
	}
}

func TestUCD(t *testing.T) {
	f, err := os.Open("./testdata/ucd/LineBreakTest.txt")
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
		line, _, _ = strings.Cut(line, "#")

		var runes []rune
		var breaks []bool
		for field := range strings.FieldsSeq(line) {
			switch field {
			case "×":
				breaks = append(breaks, false)
			case "÷":
				breaks = append(breaks, true)
			default:
				r, err := strconv.ParseInt(field, 16, 32)
				if err != nil {
					t.Fatal(err)
				}
				runes = append(runes, rune(r))
			}
		}
		if len(breaks) != len(runes)+1 {
			t.Fatalf("got %d breaks for %d runes", len(breaks), len(runes))
		}

		t.Run(fmt.Sprintf("line-%d", i), func(t *testing.T) {
			ins := linebreak.Instance{}
			ret := ins.Process(runes)
			if !slices.Equal(ret, breaks) {
				t.Fatalf("%v != %v", ret, breaks)
			}
		})
	}

	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}
