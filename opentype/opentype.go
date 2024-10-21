// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"time"
	"unsafe"
)

type PlatformID uint16
type EncodingID uint16

const (
	PlatformUnicode   PlatformID = 0
	PlatformMacintosh PlatformID = 1
	PlatformISO       PlatformID = 2
	PlatformWindows   PlatformID = 3
	PlatformCustom    PlatformID = 4
)

func sliceCast[T ~byte](from []byte) []T {
	return unsafe.Slice((*T)(unsafe.Pointer(unsafe.SliceData(from))), len(from))
}

func parseWeirdMinorVersion(buf []byte, out *uint16) int {
	// Parse the minor version component of a Version16Dot16 version. This uses digits in
	// the hex representation to encode versions. For example, 0x5000 is minor version 5.
	// How would minor version 10 be stored? I have no idea. For now we only support minor
	// versions 0 to 9.
	v := binary.BigEndian.Uint16(buf)
	if v != 0 && v < 0x1000 || v > 0x9000 {
		*out = 0
	} else {
		*out = v >> 12
	}
	return 0
}

func parsePlatformID(buf []byte, out *PlatformID) int {
	*out = PlatformID(binary.BigEndian.Uint16(buf))
	return 0
}

func parseEncodingID(buf []byte, out *EncodingID) int {
	*out = EncodingID(binary.BigEndian.Uint16(buf))
	return 0
}

func parseUint16(buf []byte, out *uint16) int {
	*out = binary.BigEndian.Uint16(buf)
	return 0
}
func parseUint32(buf []byte, out *uint32) int {
	*out = binary.BigEndian.Uint32(buf)
	return 0
}

func parseInt16(buf []byte, out *int16) int {
	*out = int16(binary.BigEndian.Uint16(buf))
	return 0
}
func parseInt32(buf []byte, out *int32) int {
	*out = int32(binary.BigEndian.Uint32(buf))
	return 0
}

func parseOffset16[T any](buf []byte, out *Offset16[T]) int {
	*out = Offset16[T](binary.BigEndian.Uint16(buf))
	return 0
}

func parseOffset32[T any](buf []byte, out *Offset32[T]) int {
	*out = Offset32[T](binary.BigEndian.Uint32(buf))
	return 0
}

func parseInt2_14(buf []byte, out *Int2_14) int {
	*out = Int2_14(binary.BigEndian.Uint16(buf))
	return 0
}

func parseInt16_16(buf []byte, out *Int16_16) int {
	*out = Int16_16(binary.BigEndian.Uint32(buf))
	return 0
}

func parseTime(buf []byte, out *time.Time) int {
	// January 1st 1904, 00:00 UTC
	const off = -2082844800
	secs := int64(binary.BigEndian.Uint64(buf))
	*out = time.Unix(secs+off, 0)
	return 0
}

func parseNameID(buf []byte, out *NameID) int {
	*out = NameID(binary.BigEndian.Uint16(buf))
	return 0
}

type Uint24 uint32

func parseUint24(buf []byte, out *Uint24) int {
	_ = buf[2]
	*out = Uint24(buf[2]) | Uint24(buf[1])<<8 | Uint24(buf[0])<<16
	return 0
}

func parseTag(buf []byte, out *Tag) int { *out = Tag(buf); return 0 }

type Tag string

func (t Tag) Uint32() uint32 {
	out := [4]byte{' ', ' ', ' ', ' '}
	copy(out[:], t)
	return binary.BigEndian.Uint32(out[:])
}

// XXX implement table checksumming

// XXX required tables: cmap, head, hhea, hmtx, maxp, name, OS/2, post

// XXX truetype outlines use cvt, fpgm, glyf, loca, prep, gasp
// XXX CFF outlines use CFF, CFF2, VORG
// XXX SVG outlines use SVG
// XXX bitmap glyphs use EBDT, EBLC, EBSC, CBDT, CBLC, sbix
// XXX advanced features use BASE, GDEF, GPOS, GSUB, JSTF, MATH
// XXX font variations use avar, cvar, fvar, gvar, HVAR, MVAR, STAT, VVAR
// XXX color fonts use COLR, CPAL, CBDT, CBLC, sbix, SVG
// XXX other tables DSIG, hdmx, kern, LTSH, MERG, meta, STAT, PCLT, VDMX, vhea, vmtx

type Offset16[T any] uint16
type Offset32[T any] uint32
type NameID uint16

func (id NameID) String() string {
	if int(id) < len(names) {
		return names[id]
	} else {
		return strconv.Itoa(int(id))
	}
}

type Int16_16 uint32

func (v Int16_16) Float() float64 { return float64(v.Integer()) + float64(v.Fraction())/float64(1<<16) }
func (v Int16_16) Integer() int   { return int(v>>16) ^ 1<<(16-1) - 1<<(16-1) }
func (v Int16_16) Fraction() int  { return int(v & ((1 << 16) - 1)) }
func (v Int16_16) String() string { return fmt.Sprintf("%d+%d/%d", v.Integer(), v.Fraction(), 1<<16) }

type Int2_14 uint16

func (v Int2_14) Float() float64 { return float64(v.Integer()) + float64(v.Fraction())/float64(1<<14) }
func (v Int2_14) Integer() int   { return int(v>>14) ^ 1<<(2-1) - 1<<(2-1) }
func (v Int2_14) Fraction() int  { return int(v & ((1 << 14) - 1)) }
func (v Int2_14) String() string { return fmt.Sprintf("%d+%d/%d", v.Integer(), v.Fraction(), 1<<14) }

// We don't limit GlyphID to 16 bit to prepare for a future where fonts can
// contain more than 65k glyphs.

type GlyphID int

type Slice[T any] []byte

/*
func (s *Slice[T]) At(i int, into *T) {
	n := s.elemSize
	s.parser(s.data[i*n:(i+1)*n], s.parentData, into)
}

func (s *Slice[T]) Len() int {
	return len(s.data) / s.elemSize
}

func (s *Slice[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := range s.Len() {
			var el T
			s.At(i, &el)
			if !yield(i, el) {
				return
			}
		}
	}
}
*/
