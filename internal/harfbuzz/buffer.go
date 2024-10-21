// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// TODO:
// Not interesting to us right now:
// hb_buffer_t * 	hb_buffer_get_empty ()
// hb_buffer_t * 	hb_buffer_reference ()
// void 	hb_buffer_add_utf32 ()
// void 	hb_buffer_add_utf16 ()
// void 	hb_buffer_add_latin1 ()
// hb_bool_t 	hb_buffer_pre_allocate ()
// void 	hb_buffer_set_unicode_funcs ()
// hb_unicode_funcs_t * 	hb_buffer_get_unicode_funcs ()
// hb_bool_t 	hb_buffer_set_length ()
// void 	hb_buffer_reverse ()
// void 	hb_buffer_reverse_range ()
// void 	hb_buffer_reverse_clusters ()
// void 	hb_buffer_set_random_state ()
// unsigned 	hb_buffer_get_random_state ()
// hb_bool_t 	(*hb_buffer_message_func_t) ()
// void 	hb_buffer_set_message_func ()
// hb_bool_t 	hb_buffer_set_user_data ()
// void * 	hb_buffer_get_user_data ()
// void 	hb_buffer_normalize_glyphs ()
// unsigned int 	hb_buffer_serialize_glyphs ()
// hb_bool_t 	hb_buffer_deserialize_glyphs ()
// unsigned int 	hb_buffer_serialize_unicode ()
// hb_bool_t 	hb_buffer_deserialize_unicode ()
// hb_buffer_serialize_format_t 	hb_buffer_serialize_format_from_string ()
// const char * 	hb_buffer_serialize_format_to_string ()
// const char ** 	hb_buffer_serialize_list_formats ()
// hb_bool_t 	hb_buffer_allocation_successful ()
// hb_buffer_t * 	hb_buffer_create_similar ()
// void 	hb_buffer_set_invisible_glyph ()
// hb_codepoint_t 	hb_buffer_get_invisible_glyph ()
// void 	hb_buffer_set_not_found_glyph ()
// hb_codepoint_t 	hb_buffer_get_not_found_glyph ()
// void 	hb_buffer_set_segment_properties ()
// void 	hb_buffer_get_segment_properties ()
// hb_buffer_diff_flags_t 	hb_buffer_diff ()
// void 	hb_buffer_set_replacement_codepoint ()
// hb_codepoint_t 	hb_buffer_get_replacement_codepoint ()
// void 	hb_buffer_set_not_found_variation_selector_glyph ()
// hb_codepoint_t 	hb_buffer_get_not_found_variation_selector_glyph ()
// hb_bool_t 	hb_segment_properties_equal ()
// unsigned int 	hb_segment_properties_hash ()
// void 	hb_segment_properties_overlay ()
// void 	hb_buffer_append ()

// #include <harfbuzz/hb.h>
// #cgo noescape hb_glyph_info_get_glyph_flags
// #cgo nocallback hb_glyph_info_get_glyph_flags
import "C"

// XXX add #cgo annotations for all other functions

import (
	"encoding/binary"
	"strings"
	"structs"
	"unsafe"

	"honnef.co/go/gutter/opentype"
	"honnef.co/go/safeish"
)

type Buffer struct {
	_ structs.HostLayout
	c C.hb_buffer_t
}

type SerializeFormat int

const (
	SerializeFormatText SerializeFormat = C.HB_BUFFER_SERIALIZE_FORMAT_TEXT
	SerializeFormatJSON SerializeFormat = C.HB_BUFFER_SERIALIZE_FORMAT_JSON
)

type SerializeFlags int

const (
	SerializeFlagsDefault      SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_DEFAULT
	SerializeFlagsNoClusters   SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_NO_CLUSTERS
	SerializeFlagsNoPositions  SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_NO_POSITIONS
	SerializeFlagsNoGlyphNames SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_NO_GLYPH_NAMES
	SerializeFlagsGlyphExtents SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_GLYPH_EXTENTS
	SerializeFlagsGlyphFlags   SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_GLYPH_FLAGS
	SerializeFlagsNoAdvances   SerializeFlags = C.HB_BUFFER_SERIALIZE_FLAG_NO_ADVANCES
)

type ContentType int

const (
	ContentTypeInvalid ContentType = C.HB_BUFFER_CONTENT_TYPE_INVALID
	ContentTypeUnicode ContentType = C.HB_BUFFER_CONTENT_TYPE_UNICODE
	ContentTypeGlyphs  ContentType = C.HB_BUFFER_CONTENT_TYPE_GLYPHS
)

type ClusterLevel int

const (
	ClusterLevelMonotoneGraphemes  ClusterLevel = C.HB_BUFFER_CLUSTER_LEVEL_MONOTONE_GRAPHEMES
	ClusterLevelMonotoneCharacters ClusterLevel = C.HB_BUFFER_CLUSTER_LEVEL_MONOTONE_CHARACTERS
	ClusterLevelCharacters         ClusterLevel = C.HB_BUFFER_CLUSTER_LEVEL_CHARACTERS
	ClusterLevelDefault            ClusterLevel = C.HB_BUFFER_CLUSTER_LEVEL_DEFAULT
)

type BufferFlags int

const (
	BufferFlagsDefault                    BufferFlags = C.HB_BUFFER_FLAG_DEFAULT
	BufferFlagsBOT                        BufferFlags = C.HB_BUFFER_FLAG_BOT
	BufferFlagsEOT                        BufferFlags = C.HB_BUFFER_FLAG_EOT
	BufferFlagsPreserveDefaultIgnorables  BufferFlags = C.HB_BUFFER_FLAG_PRESERVE_DEFAULT_IGNORABLES
	BufferFlagsRemoveDefaultIgnorables    BufferFlags = C.HB_BUFFER_FLAG_REMOVE_DEFAULT_IGNORABLES
	BufferFlagsDoNotInsertDottedCircle    BufferFlags = C.HB_BUFFER_FLAG_DO_NOT_INSERT_DOTTED_CIRCLE
	BufferFlagsVerify                     BufferFlags = C.HB_BUFFER_FLAG_VERIFY
	BufferFlagsProduceUnsafeToConcat      BufferFlags = C.HB_BUFFER_FLAG_PRODUCE_UNSAFE_TO_CONCAT
	BufferFlagsProduceSafeToInsertTatweel BufferFlags = C.HB_BUFFER_FLAG_PRODUCE_SAFE_TO_INSERT_TATWEEL
)

type GlyphFlags int

const (
	GlyphFlagsUnsafeToBreak       GlyphFlags = C.HB_GLYPH_FLAG_UNSAFE_TO_BREAK
	GlyphFlagsUnsafeToConcat      GlyphFlags = C.HB_GLYPH_FLAG_UNSAFE_TO_CONCAT
	GlyphFlagsSafeToInsertTatweel GlyphFlags = C.HB_GLYPH_FLAG_SAFE_TO_INSERT_TATWEEL
)

type GlyphInfo struct {
	_ structs.HostLayout

	Codepoint int32
	_         uint32
	Cluster   int32
	_         uint32
	_         uint32
}

func (gi *GlyphInfo) Flags() GlyphFlags {
	return GlyphFlags(C.hb_glyph_info_get_glyph_flags(safeish.Cast[*C.hb_glyph_info_t](gi)))
}

type GlyphPosition struct {
	_ structs.HostLayout

	XAdvance int32
	YAdvance int32
	XOffset  int32
	YOffset  int32

	_ uint32
}

func NewBuffer() *Buffer {
	return safeish.Cast[*Buffer](C.hb_buffer_create())
}

func (b *Buffer) Destroy() { C.hb_buffer_destroy(&b.c) }

func (b *Buffer) Reset() { C.hb_buffer_reset(&b.c) }

func (b *Buffer) Clear() { C.hb_buffer_clear_contents(&b.c) }

func (b *Buffer) AddString(s string, offset, length int) {
	if length == -1 {
		length = len(s)
	}
	offset = min(len(s), offset)
	length = min(len(s)-offset, length)
	C.hb_buffer_add_utf8(
		&b.c,
		safeish.Cast[*C.char](unsafe.StringData(s)),
		C.int(len(s)),
		C.uint(offset),
		C.int(length),
	)
}

func (b *Buffer) AddRunes(r []rune, offset, length int) {
	offset = min(len(r), offset)
	length = min(len(r)-offset, length)
	C.hb_buffer_add_codepoints(
		&b.c,
		safeish.Cast[*C.hb_codepoint_t](&r[0]),
		C.int(len(r)),
		C.uint(offset),
		C.int(length),
	)
}

func (b *Buffer) AddRune(r rune, cluster uint) {
	C.hb_buffer_add(&b.c, C.hb_codepoint_t(r), C.uint(cluster))
}

func (b *Buffer) SetContentType(typ ContentType) {
	C.hb_buffer_set_content_type(&b.c, C.hb_buffer_content_type_t(typ))
}

func (b *Buffer) ContentType() ContentType {
	return ContentType(C.hb_buffer_get_content_type(&b.c))
}

func (b *Buffer) SetDirection(dir Direction) {
	C.hb_buffer_set_direction(&b.c, C.hb_direction_t(dir))
}

func (b *Buffer) Direction() Direction {
	return Direction(C.hb_buffer_get_direction(&b.c))
}

func (b *Buffer) SetScript(s opentype.Tag) {
	C.hb_buffer_set_script(&b.c, C.hb_script_t(s.Uint32()))
}

func (b *Buffer) Script() opentype.Tag {
	tag := C.hb_buffer_get_script(&b.c)
	out := [4]byte{' ', ' ', ' ', ' '}
	binary.BigEndian.PutUint32(out[:], uint32(tag))
	return opentype.Tag(strings.TrimRight(string(out[:]), " "))
}

func (b *Buffer) SetLanguage(lang Language) {
	C.hb_buffer_set_language(&b.c, lang.c)
}

func (b *Buffer) Language() Language {
	return Language{
		c: C.hb_buffer_get_language(&b.c),
	}
}

func (b *Buffer) Length() int {
	return int(C.hb_buffer_get_length(&b.c))
}

func (b *Buffer) GuessSegmentProperties() {
	C.hb_buffer_guess_segment_properties(&b.c)
}

func (b *Buffer) String() string {
	return b.serialize(
		0,
		-1,
		nil,
		SerializeFormatText,
		SerializeFlagsDefault|SerializeFlagsGlyphFlags,
	)
}

func (b *Buffer) MarshalJSON() ([]byte, error) {
	s := b.serialize(
		0,
		-1,
		nil,
		SerializeFormatText,
		SerializeFlagsDefault|SerializeFlagsGlyphFlags,
	)

	return unsafe.Slice(unsafe.StringData(s), len(s)), nil
}

func (b *Buffer) serialize(
	start int,
	end int,
	font *Font,
	format SerializeFormat,
	flags SerializeFlags,
) string {
	buf := make([]byte, 4096)
	var n C.uint
	var fontc *C.hb_font_t
	if font != nil {
		fontc = &font.c
	}
	for {
		C.hb_buffer_serialize(
			&b.c,
			0,
			^C.uint(0),
			safeish.Cast[*C.char](&buf[0]),
			C.uint(len(buf)),
			&n,
			fontc,
			C.hb_buffer_serialize_format_t(format),
			C.hb_buffer_serialize_flags_t(flags),
		)

		if int(n) < len(buf) {
			return unsafe.String(&buf[0], len(buf))
		}
		buf = make([]byte, len(buf)*2)
	}
}

func (b *Buffer) SetClusterLevel(level ClusterLevel) {
	C.hb_buffer_set_cluster_level(&b.c, C.hb_buffer_cluster_level_t(level))
}

func (b *Buffer) ClusterLevel() ClusterLevel {
	return ClusterLevel(C.hb_buffer_get_cluster_level(&b.c))
}

func (b *Buffer) SetFlags(flags BufferFlags) {
	C.hb_buffer_set_flags(&b.c, C.hb_buffer_flags_t(flags))
}

func (b *Buffer) Flags() BufferFlags {
	return BufferFlags(C.hb_buffer_get_flags(&b.c))
}

func (b *Buffer) GlyphInfos() []GlyphInfo {
	var n C.uint
	ptr := C.hb_buffer_get_glyph_infos(&b.c, &n)
	return unsafe.Slice(safeish.Cast[*GlyphInfo](ptr), n)
}

func (b *Buffer) GlyphPositions() []GlyphPosition {
	var n C.uint
	ptr := C.hb_buffer_get_glyph_positions(&b.c, &n)
	return unsafe.Slice(safeish.Cast[*GlyphPosition](ptr), n)
}

func (b *Buffer) HasPositions() bool {
	return C.hb_buffer_has_positions(&b.c) != 0
}

func (b *Buffer) ReverseClusters() {
	C.hb_buffer_reverse_clusters(&b.c)
}
