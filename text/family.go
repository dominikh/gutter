// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build ignore

package text

import (
	"strings"

	xlanguage "honnef.co/go/gutter/internal/language"
	"honnef.co/go/stuff/container/maybe"
)

// TODO Document how
// - https://github.com/w3c/csswg-drafts/issues/4397
// - https://github.com/w3c/i18n-discuss/wiki/Generic-font-families
// affected our choices for FontGenre.

type FontGenre int

const (
	NoGenre FontGenre = iota
	// UI defaults to the most appropriate or common genre for displaying UI in a given
	// script and language. Most of the time, that is Sans.
	UI
	Sans
	Serif
	Cursive
	Monospace
	Emoji // XXX figure out fate of this
	Kufic // XXX figure out fate of this
	Rashi // XXX figure out fate of this
	// Kai is a calligraphic style for Chinese.
	Kai
	// Fang Song is an intermediate form between Song and Kai.
	FangSong
	Nastaliq
	// Math selects a combination of latin, greek, math, and symbol fonts suitable for
	// displaying math.
	Math

	// Hei is the Chinese equivalent to Sans.
	Hei = Sans
	// Gothic is the Japanese equivalent to Sans.
	Gothic = Sans
	// Gulim is the Korean equivalent to Sans.
	Gulim = Sans
	// The Arabic Kufi style roughly maps to Sans.
	Kufi = Sans

	// Song is the Chinese equivalent to Serif.
	Song = Serif
	// Ming is a synonym for Song.
	Ming = Song
	// Mincho is the Japanese synonym for Song.
	Mincho = Song
	// Batang is the Korean equivalent to Serif.
	Batang = Serif
	// The Arabic Naskh style roughly maps to Serif.
	Naskh = Serif
)

func Tag(s string) xlanguage.Tag {
	return xlanguage.MustParse(s)
}

var (
	hans = xlanguage.MustParseScript("hans")
	hant = xlanguage.MustParseScript("hant")
	hani = xlanguage.MustParseScript("hani")
)

// Canonicalize canonicalizes language tags for use with font maps. It discards
// information that may be relevant to other localization tasks but is irrelevant for font
// selection.
func Canonicalize(tag xlanguage.Tag) xlanguage.Tag {
	switch tag.LangID {
	case base("cmn"): // Mandarin
		tag.LangID = base("zh")
	case base("yue"): // Cantonese
		tag.LangID = base("zh")
	}
	return tag
}

func SomeTag(s string) maybe.Option[xlanguage.Tag] {
	return maybe.Some(Tag(s))
}

func base(s string) xlanguage.Language {
	return xlanguage.MustParseBase(s)
}

type FontFamilyKey struct {
	Tag   xlanguage.Tag
	Genre FontGenre
}

type FontFamilies map[FontFamilyKey][]string

func (ff FontFamilies) lookupFontNames(tag xlanguage.Tag, script xlanguage.Script, genre FontGenre) []string {
	if tag.ScriptID == 0 {
		if script == 0 {
			tag.ScriptID = tag.LangID.SuppressScript()
		} else {
			tag.ScriptID = script
		}
	}

	// We intentionally drop the variant and extension as we don't use those in
	// FontFamilies.
	tag = Canonicalize(xlanguage.Tag{
		LangID:   tag.LangID,
		ScriptID: tag.ScriptID,
		RegionID: tag.RegionID,
	})

	if genre == NoGenre {
		genre = UI
	}

	try := func(tag xlanguage.Tag) ([]string, bool) {
		if fonts, ok := ff[FontFamilyKey{tag, genre}]; ok {
			return fonts, true
		} else if genre == UI {
			if fonts, ok := ff[FontFamilyKey{tag, Sans}]; ok {
				return fonts, true
			}
		} else {
			if fonts, ok := ff[FontFamilyKey{tag, NoGenre}]; ok {
				return fonts, true
			}
		}
		return nil, false
	}

	if names, ok := try(tag); ok {
		return names
	}

	// Try stripping the region first
	if tag.RegionID != 0 {
		tag.RegionID = 0
		if names, ok := try(tag); ok {
			return names
		}
	}

	// Then try stripping the language.
	if tag.LangID != 0 {
		tag.LangID = 0
		if names, ok := try(tag); ok {
			return names
		}
	}

	// Finally, fall back to und.
	xlanguage.Und.Parent()
	names, _ := try(xlanguage.Und)
	return names
}

func (ff FontFamilies) Candidates(
	tag xlanguage.Tag,
	script xlanguage.Script,
	genre FontGenre,
) []string {
	var out []string

	resolve := func(name string) (xlanguage.Tag, FontGenre, bool) {
		const prefix = "default:"
		if !strings.HasPrefix(name, prefix) {
			return xlanguage.Tag{}, NoGenre, false
		}
		name = name[len(prefix):]
		genre := genre
		newName := name
		// if n := strings.Index(name, "/"); n >= 0 {
		// 	genre = genres[name[n+1:]]
		// 	newName = name[:n]
		// }
		return Tag(newName), genre, true
	}
	loadGroup := func(group []string) {
		for _, name := range group {
			if newTag, newGenre, ok := resolve(name); ok {
				// FIXME(dh): detect cycles
				out = append(out, ff.Candidates(newTag, 0, newGenre)...)
			} else {
				out = append(out, name)
			}
		}
	}

	group := ff.lookupFontNames(tag, script, genre)
	loadGroup(group)

	return out
}
