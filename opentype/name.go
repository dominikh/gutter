// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import (
	"encoding/binary"
	"iter"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"honnef.co/go/stuff/container/maybe"

	"golang.org/x/text/language"
)

const (
	NameCopyright                      NameID = 0
	NameFontFamilyName                 NameID = 1
	NameFontSubfamilyName              NameID = 2
	NameFontIdentifier                 NameID = 3
	NameFullFontName                   NameID = 4
	NameVersion                        NameID = 5
	NamePostScriptName                 NameID = 6
	NameTrademark                      NameID = 7
	NameManufacturer                   NameID = 8
	NameDesigner                       NameID = 9
	NameDescription                    NameID = 10
	NameVendorURL                      NameID = 11
	NameDesignerURL                    NameID = 12
	NameLicenseDescription             NameID = 13
	NameLicenseURL                     NameID = 14
	_                                  NameID = 15
	NameTypographicFamilyName          NameID = 16
	NameTypographicSubfamilyName       NameID = 17
	NameCompatibleFullName             NameID = 18
	NameSampleText                     NameID = 19
	NamePostScriptCID                  NameID = 20
	NameWWSFamilyName                  NameID = 21
	NameWWSSubfamilyName               NameID = 22
	NameLightBackgroundPalette         NameID = 23
	NameDarkBackgroundPalette          NameID = 24
	NamePostScriptVariationsNamePrefix NameID = 25
)

var names = [...]string{
	NameCopyright:                      "copyright(0)",
	NameFontFamilyName:                 "font family name(1)",
	NameFontSubfamilyName:              "font subfamily name(2)",
	NameFontIdentifier:                 "font identifier (3)",
	NameFullFontName:                   "full font name (4)",
	NameVersion:                        "version (5)",
	NamePostScriptName:                 "PostScript name (6)",
	NameTrademark:                      "trademark (7)",
	NameManufacturer:                   "manufacturer (8)",
	NameDesigner:                       "designer (9)",
	NameDescription:                    "description (10)",
	NameVendorURL:                      "vendor URL (11)",
	NameDesignerURL:                    "designer URL (12)",
	NameLicenseDescription:             "license description (13)",
	NameLicenseURL:                     "license URL (14)",
	NameTypographicFamilyName:          "typogrpahic family name (16)",
	NameTypographicSubfamilyName:       "typographic subfamily name (17)",
	NameCompatibleFullName:             "compatible full name (18)",
	NameSampleText:                     "sample text (19)",
	NamePostScriptCID:                  "PostScript CID (20)",
	NameWWSFamilyName:                  "WWS family name (21)",
	NameWWSSubfamilyName:               "WWS subfamily name (22)",
	NameLightBackgroundPalette:         "light background palette (23)",
	NameDarkBackgroundPalette:          "dark background palette (24)",
	NamePostScriptVariationsNamePrefix: "PostScript variations name prefix (25)",
}

func decodeUTF16BEBMP(data []byte) string {
	// TODO(dh): should we just use decodeUTF16BE and silently accept non-BMP data?
	var out strings.Builder
	for i := 0; i < len(data)-1; i += 2 {
		r := rune(binary.BigEndian.Uint16(data[i : i+2]))
		out.WriteRune(r)
	}
	if len(data)%2 != 0 {
		out.WriteRune(utf8.RuneError)
	}
	return out.String()
}

func decodeUTF16BE(data []byte) string {
	var out strings.Builder
	var surr rune
	for i := 0; i < len(data)-1; i += 2 {
		r := rune(binary.BigEndian.Uint16(data[i : i+2]))
		if surr != 0 {
			out.WriteRune(utf16.DecodeRune(surr, r))
			surr = 0
		} else if utf16.IsSurrogate(r) {
			surr = r
		} else {
			out.WriteRune(r)
		}
	}
	if len(data)%2 != 0 || surr != 0 {
		out.WriteRune(utf8.RuneError)
	}
	return out.String()
}

func (rec *NameRecord) Decodable() bool {
	// In theory, we can decode any concrete string encoded in an encoding that is an
	// extended ASCII encoding if the string is ASCII-only. In practice, no modern fonts
	// should be using any of those encodings.

	return rec.PlatformID == PlatformUnicode ||
		(rec.PlatformID == PlatformWindows && (rec.EncodingID == EncodingWindowsUnicodeBMP || rec.EncodingID == EncodingWindowsUnicodeFullRepertoire)) ||
		(rec.PlatformID == PlatformMacintosh && rec.EncodingID == EncodingMacintoshRoman)
}

type ExtendedLanguageRange string

// A NameFilterQuery specifies the filter for use with [NameTable.Filter].
type NameFilterQuery struct {
	// Only find names with this ID.
	Name maybe.Option[NameID]
	// Only find names with this language. Languages are compared for exact equality, with
	// no fallbacks or semantics taken into account. For better language-matching, see
	// [NameTable.ComputeLookups].
	Language maybe.Option[language.Tag]
	// Include names that use encodings not supported by [NameTable.DecodeName].
	NonDecodable bool
}

// Filter returns an iterator over all names in the table that match the given query.
func (tbl *NameTable) Filter(q NameFilterQuery) iter.Seq[NameRecord] {
	// OPT(dh): if q.NonDecodable is false, only scan groups of supported encodings
	// OPT(dh): if q.Language is set, only scan groups of supported languages
	// OPT(dh): if q.Name is set, use binary search in each group to find name
	return func(yield func(v NameRecord) bool) {
		name, haveName := q.Name.Get()
		for _, rec := range tbl.NameRecords() {
			if haveName && name != rec.NameID {
				continue
			}
			if !q.NonDecodable && !rec.Decodable() {
				continue
			}
			if !yield(rec) {
				break
			}
		}
	}
}

func (tbl *NameTable) langTag(langID uint16) language.Tag {
	if langID < 0x8000 || int(langID)-0x8000 >= int(tbl.NumLangTagRecords()) {
		return language.Und
	}
	lang := tbl.LangTagRecord(int(langID) - 0x8000)
	start := int(lang.Offset)
	end := start + int(lang.Length)
	if start >= len(tbl.Data) || end >= len(tbl.Data) {
		return language.Und
	}
	return language.MustParse(decodeUTF16BE(tbl.Data[start:end]))
}

// Language returns the language tag of name.
func (tbl *NameTable) Language(name *NameRecord) language.Tag {
	switch name.PlatformID {
	case PlatformUnicode:
		return tbl.langTag(name.LanguageID)
	case PlatformMacintosh:
		if name.LanguageID < 0x800 {
			if int(name.LanguageID) >= len(MacintoshLanguageIDs) {
				return language.Und
			}
			return MacintoshLanguageIDs[int(name.LanguageID)]
		} else {
			return tbl.langTag(name.LanguageID)
		}
	case PlatformWindows:
		if name.LanguageID < 0x8000 {
			return MicrosoftLocaleIDs[int(name.LanguageID)]
		} else {
			return tbl.langTag(name.LanguageID)
		}
	default:
		return language.Und
	}
}
