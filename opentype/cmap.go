// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import (
	"cmp"
	"sort"
)

// From https://learn.microsoft.com/en-us/typography/opentype/spec/cmap:
//
// Apart from a format 14 subtable, all other subtables are exclusive: applications should
// select and use one and ignore the others. If a Unicode subtable is used (platform 0, or
// platform 3 / encoding 1 or 10), then a format 14 subtable using platform 0/encoding 5
// can also be supplemented for mapping Unicode Variation Sequences.
//
// If a font includes Unicode subtables for both 16-bit encoding (typically, format 4) and
// also 32-bit encoding (formats 10 or 12), then the characters supported by the subtable
// for 32-bit encoding should be a superset of the characters supported by the subtable
// for 16-bit encoding, and the 32-bit encoding should be used by applications.
//
// Fonts should not include 16-bit Unicode subtables using both format 4 and format 6;
// format 4 should be used. Similarly, fonts should not include 32-bit Unicode subtables
// using both format 10 and format 12; format 12 should be used.
//
// If a font includes encoding records for Unicode subtables of the same format but with
// different platform IDs, an application may choose which to select, but should make this
// selection consistently each time the font is used.
//
// [...]
//
// The language field must be set to zero for all 'cmap' subtables whose platform IDs are
// other than Macintosh (platform ID 1). For 'cmap' subtables whose platform IDs are
// Macintosh, set this field to the Macintosh language ID of the 'cmap' subtable plus one,
// or to zero if the 'cmap' subtable is not language-specific. For example, a Mac OS
// Turkish 'cmap' subtable must set this field to 18, since the Macintosh language ID for
// Turkish is 17. A Mac OS Roman 'cmap' subtable must set this field to 0, since Mac OS
// Roman is not a language-specific encoding.

// Gutter-specific invariants:
//
// - the only macOS encoding we support is Roman with no language, so we only support
//   tables with Language == 0.

func (cmap *CmapSubtable) Lookup(r rune) GlyphID {
	switch cmap.Format {
	case 0:
		panic("not implemented")
	case 4:
		return cmap.Format4.Lookup(r)
	case 6:
		panic("not implemented")
	case 10:
		panic("not implemented")
	case 12:
		return cmap.Format12.Lookup(r)
	case 13:
		return cmap.Format13.Lookup(r)
	case 14:
		panic("not implemented")
	default:
		return 0
	}
}

func (tbl *CmapSubtableFormat4) Lookup(r rune) GlyphID {
	n := tbl.NumEndCodes()
	i := sort.Search(n, func(i int) bool {
		return rune(tbl.EndCode(i)) >= r
	})

	// For r <= 0xFFFF, this can only happen for malformed fonts, as endCodes is required
	// to contain an entry for 0xFFFF. For r > 0xFFFF, this always happens because we run
	// the binary search unconditionally.
	if i == n {
		return 0
	}

	startCode := tbl.StartCode(i)
	if startCode > uint16(r) {
		return 0
	}
	idDelta := tbl.IDDelta(i)
	idRangeOffset := tbl.IDRangeOffset(i)

	if idRangeOffset != 0 {
		// idRangeOffset specifies the byte offset from the location of
		// idRangeOffsets[segment] in the font file to the beginning of the glyph ID array
		// for the segment. We convert it to an index into glyphIDs, which contains the
		// glyph IDs for all segments, by converting the byte offset into a uint16 offset
		// and subtracting the number of remaining items in idRangeOffsets. This gives us
		// the index into glyphIDs for the first item for the segment.
		start := int(idRangeOffset/2) - (tbl.NumIdRangeOffsets() - i)
		index := uint16(r) - startCode
		id := tbl.GlyphID(start + int(index))
		if id == 0 {
			return 0
		}
		return GlyphID(id + uint16(idDelta))
	} else {
		return GlyphID(uint16(idDelta) + uint16(r))
	}
}

func (tbl *CmapSubtableFormat12) Lookup(r rune) GlyphID {
	n := tbl.NumGroups()
	i := sort.Search(n, func(i int) bool {
		return rune(tbl.Group(i).EndCharCode) >= r
	})
	if i == n {
		return 0
	}

	group := tbl.Group(i)
	if rune(group.StartCharCode) > r {
		return 0
	}

	return GlyphID(r - rune(group.StartCharCode) + rune(group.StartGlyphID))
}

func (tbl *CmapSubtableFormat13) Lookup(r rune) GlyphID {
	n := tbl.NumGroups()
	i := sort.Search(n, func(i int) bool {
		return rune(tbl.Group(i).EndCharCode) >= r
	})
	if i == n {
		return 0
	}

	group := tbl.Group(i)
	if rune(group.StartCharCode) > r {
		return 0
	}

	return GlyphID(group.GlyphID)
}

// Table preference (preferring 32-bit over 16 over 8, sorted by how likely we
// are to find it in the font):
//
// "Each platform ID, platform-specific encoding ID, and subtable language combination may
// appear only once in the 'cmap' table." Therefore we don't have to select from multiple
// formats for the same encoding, and can solely select based on the best encoding (32-bit
// over 16-bit over 8-bit.)

// 32-bit Unicode
// platformWindows, msEncUnicodeFullRepertoire
// platformUnicode, unicode2_0
// platformUnicode, unicode_format13

// 16-bit Unicode
// platformWindows, msEncUnicodeBMP
// platformUnicode, unicode2_0_bmp
// We intentionally don't support Unicode 1 because we'd have to remap some runes

// 8-bit extended ASCII
// platformMacintosh, macEncRoman, language 0

// platformUnicode, unicode_varseq -- table can coexist with with 32-bit and 16-bit unicode tables

// Records are sorted by (platform, encoding, language)

// 3 expected platforms

// platformUnicode: 2 expected encodings
// platformWindows: 2 expected encodings
// platformMac: 1 expected encoding

var desiredEncodingsLookup = [...][11]int8{
	PlatformUnicode: [11]int8{
		EncodingUnicode20:       128 - 1,
		EncodingUnicodeFormat13: 128 - 2,
		EncodingUnicode20BMP:    128 - 3,
	},
	PlatformWindows: [11]int8{
		EncodingWindowsUnicodeBMP:            128 - 3,
		EncodingWindowsUnicodeFullRepertoire: 128 - 1,
	},
	PlatformMacintosh: [11]int8{
		EncodingMacintoshRoman: 128 - 5,
	},
}

var desiredEncodings = [...]struct {
	platform PlatformID
	encoding EncodingID
}{
	{PlatformWindows, EncodingWindowsUnicodeFullRepertoire},
	{PlatformUnicode, EncodingUnicode20},
	{PlatformUnicode, EncodingUnicodeFormat13},

	{PlatformWindows, EncodingWindowsUnicodeBMP},
	{PlatformUnicode, EncodingUnicode20BMP},

	{PlatformMacintosh, EncodingMacintoshRoman},
}

// SelectEncoding selects the best available encoding.
func (cmap *CmapTable) SelectEncoding() (EncodingRecord, bool) {
	const alwaysUseBinarySearch = false

	if cmap.NumEncodingRecords() < 16 && !alwaysUseBinarySearch {
		// The font has an expectedly small number of cmap subtables, so just do a linear scan
		// to find the best encoding.
		selectedIdx := -1
		selectedRank := int8(-1)
		for i, rec := range cmap.EncodingRecords() {
			if int(rec.PlatformID) >= len(desiredEncodingsLookup) {
				continue
			}
			if int(rec.EncodingID) >= len(desiredEncodingsLookup[0]) {
				continue
			}
			if n := desiredEncodingsLookup[rec.PlatformID][rec.EncodingID] - 1; n > selectedRank {
				// XXX handle macintosh platform language
				selectedRank = n
				selectedIdx = i
			}
		}
		if selectedIdx == -1 {
			return EncodingRecord{}, false
		}
		return cmap.EncodingRecord(selectedIdx), true
	} else {
		for _, enc := range desiredEncodings {
			i, ok := sort.Find(cmap.NumEncodingRecords(), func(i int) int {
				rec := cmap.EncodingRecord(i)
				if n := cmp.Compare(enc.platform, rec.PlatformID); n != 0 {
					return n
				}
				return cmp.Compare(enc.encoding, rec.EncodingID)
			})
			if ok {
				// XXX handle macintosh platform language
				return cmap.EncodingRecord(i), true
			}
		}
		return EncodingRecord{}, false
	}
}
