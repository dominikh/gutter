// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentypehl

import (
	"errors"
	"iter"

	"honnef.co/go/stuff/container/maybe"
	"honnef.co/go/gutter/opentype"

	"golang.org/x/text/language"
)

type File struct {
	data      []byte
	directory opentype.TableDirectory
	// OPT(dh): store the relevant union member to avoid a branch on every lookup
	Cmap opentype.CmapSubtable
}

func (f *File) Raw() *opentype.TableDirectory {
	return &f.directory
}

func NewFile(data []byte) (*File, error) {
	f := File{
		data: data,
	}
	opentype.ParseTableDirectory(f.data, &f.directory)

	if rec, ok := f.directory.FindTable("cmap"); ok {
		var cmap opentype.CmapTable
		opentype.ParseCmapTable(rec.Data(), &cmap)
		if erec, ok := cmap.SelectEncoding(); ok {
			if !erec.Subtable(&f.Cmap) {
				return nil, errors.New("invalid cmap record with nil subtable")
			}
		} else {
			return nil, errors.New("no supported cmap encoding found")
		}
	} else {
		return nil, errors.New("missing required cmap table")
	}
	return &f, nil
}

type FamilyKind int

const (
	TypographicFamily FamilyKind = iota + 1
	WWSFamily
	RBIBIFamily
)

func (kind FamilyKind) String() string {
	switch kind {
	case TypographicFamily:
		return "typographic family"
	case WWSFamily:
		return "WWS family"
	case RBIBIFamily:
		return "R/B/I/BI family"
	default:
		return "invalid family"
	}
}

// Family returns the font family.
func (f *File) Family(lang language.Tag) (family, subfamily string, kind FamilyKind) {
	names := f.Names()
	if f, ok := names.Lookup(opentype.NameTypographicFamilyName, lang); ok {
		family = f.String()
		if s, ok := names.Lookup(opentype.NameTypographicSubfamilyName, lang); ok {
			subfamily = s.String()
		}
		return family, subfamily, TypographicFamily
	} else if f, ok := names.Lookup(opentype.NameWWSFamilyName, lang); ok {
		family = f.String()
		if s, ok := names.Lookup(opentype.NameWWSSubfamilyName, lang); ok {
			subfamily = s.String()
		}
		return family, subfamily, WWSFamily
	} else if f, ok := names.Lookup(opentype.NameFontFamilyName, lang); ok {
		family = f.String()
		if s, ok := names.Lookup(opentype.NameFontSubfamilyName, lang); ok {
			subfamily = s.String()
		}
		return family, subfamily, RBIBIFamily
	} else {
		return "", "", 0
	}
}

func (f *File) Names() *NameTable {
	if rec, ok := f.directory.FindTable("name"); ok {
		var out NameTable
		opentype.ParseNameTable(rec.Data(), &out.raw)
		return &out
	} else {
		// XXX return error
		panic("malformed font")
	}
}

type NameTable struct {
	raw opentype.NameTable
}

// ComputeLookups computes a lookup table for efficient lookups of names with preferred
// languages.
//
// It allocates memory proportional to the number of names in the table.
func (tbl *NameTable) ComputeLookups() NameLookups {
	entries := map[opentype.NameID]*nameLookupsEntry{}
	for rec := range tbl.raw.Filter(opentype.NameFilterQuery{}) {
		entry := entries[rec.NameID]
		if entry == nil {
			entry = &nameLookupsEntry{}
			entries[rec.NameID] = entry
		}
		entry.langs = append(entry.langs, tbl.raw.Language(&rec))
		entry.recs = append(entry.recs, rec)
	}
	for _, entry := range entries {
		entry.matcher = language.NewMatcher(entry.langs, language.PreferSameScript(true))
	}

	out := map[opentype.NameID]nameLookupsEntry{}
	for k, v := range entries {
		out[k] = *v
	}
	return NameLookups{
		tbl:     tbl,
		entries: out,
	}
}

func (tbl *NameTable) Lookup(name opentype.NameID, lang language.Tag) (Name, bool) {
	var langs []language.Tag
	var recs []opentype.NameRecord
	for rec := range tbl.raw.Filter(opentype.NameFilterQuery{Name: maybe.Some(name)}) {
		langs = append(langs, tbl.raw.Language(&rec))
		recs = append(recs, rec)
	}
	if len(langs) == 0 {
		return Name{}, false
	}
	m := language.NewMatcher(langs, language.PreferSameScript(true))
	_, idx, _ := m.Match(lang)

	return Name{
		tbl: tbl,
		raw: recs[idx],
	}, true
}

func (tbl *NameTable) All() iter.Seq[Name] {
	return func(yield func(Name) bool) {
		for name := range tbl.raw.Filter(opentype.NameFilterQuery{}) {
			if !yield(Name{tbl, name}) {
				break
			}
		}
	}
}

// NameLookups provides efficient lookups of language-tagged names. It can be created
// using [NameTable.ComputeLookups].
type NameLookups struct {
	tbl     *NameTable
	entries map[opentype.NameID]nameLookupsEntry
}

type nameLookupsEntry struct {
	langs   []language.Tag
	matcher language.Matcher
	recs    []opentype.NameRecord
}

// Lookup finds a translation of name that is the best fit given the preferred languages.
// It returns false only if name doesn't exist in any language.
func (nl *NameLookups) Lookup(name opentype.NameID, preferredLanguages ...language.Tag) (Name, bool) {
	entry, ok := nl.entries[name]
	if !ok {
		return Name{}, false
	}
	_, idx, _ := entry.matcher.Match(preferredLanguages...)
	return Name{tbl: nl.tbl, raw: entry.recs[idx]}, true
}

type Name struct {
	tbl *NameTable
	raw opentype.NameRecord
}

func (n Name) Language() language.Tag { return n.tbl.raw.Language(&n.raw) }
func (n Name) Decodable() bool        { return n.raw.Decodable() }
func (n Name) Length() int            { return int(n.raw.Length) }
func (n Name) ID() opentype.NameID    { return n.raw.NameID }

// String returns the UTF-8-encoded string of the name described by name. It only
// supports names encoded using Unicode or the MacRoman encoding. It returns the empty
// string for unsupported encodings.
func (n Name) String() string {
	if n.tbl == nil {
		return "<invalid name>"
	}
	data := n.tbl.raw.Data[n.raw.StringOffset : int(n.raw.StringOffset)+int(n.raw.Length)]
	dec, _ := opentype.Decode(data, opentype.Encoding{PlatformID: n.raw.PlatformID, EncodingID: n.raw.EncodingID})
	return dec
}
