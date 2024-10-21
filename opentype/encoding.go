// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import (
	"fmt"

	"golang.org/x/text/encoding/charmap"
)

type UnknownEncodingError struct {
	Encoding Encoding
}

func (err UnknownEncodingError) Error() string {
	return fmt.Sprintf("unknown encoding %s", err.Encoding)
}

type Encoding struct {
	PlatformID PlatformID
	EncodingID EncodingID
}

func supportedEncoding(enc Encoding) bool {
	switch enc {
	case Encoding{PlatformUnicode, EncodingUnicode20BMP}: // Unicode 2.0+, BMP
		return true
	case Encoding{PlatformUnicode, EncodingUnicode20}: // Unicode 2.0+
		return true
	case Encoding{PlatformMacintosh, EncodingMacintoshRoman}:
		return true
	case Encoding{PlatformWindows, EncodingWindowsSymbol}:
		return false
	case Encoding{PlatformWindows, EncodingWindowsUnicodeBMP}:
		return true
	case Encoding{PlatformWindows, EncodingWindowsUnicodeFullRepertoire}:
		return true
	default:
		return false
	}
}

func (enc Encoding) String() string {
	return fmt.Sprintf("%d/%d", enc.PlatformID, enc.EncodingID)
}

func Decode(data []byte, enc Encoding) (string, error) {
	// We intentionally only support Unicode-based encodings, as well as MacRoman for a
	// minimum of compatibility with very old fonts. Even then, we ignore that MacRoman
	// can refer to 8 different encodings that get selected based on the language ID. We
	// don't see any value in supporting 30 years old legacy encodings.
	//
	// Even if we wanted to support all of the old Macintosh encodings, the majority of
	// them aren't documented.
	//
	// See https://www.unicode.org/Public/MAPPINGS/VENDORS/APPLE/Readme.txt for details on
	// Macintosh encodings.
	//
	// Similarly, we don't support any of the Microsoft-specific encodings, other than the
	// ones based on Unicode. For example, we do not support Big5.

	switch enc {
	case Encoding{PlatformUnicode, 3}: // Unicode 2.0+, BMP
		return decodeUTF16BEBMP(data), nil
	case Encoding{PlatformUnicode, 4}: // Unicode 2.0+
		return decodeUTF16BE(data), nil
	case Encoding{PlatformMacintosh, EncodingMacintoshRoman}:
		out, err := charmap.Macintosh.NewDecoder().Bytes(data)
		return string(out), err
	case Encoding{PlatformWindows, EncodingWindowsSymbol}:
		// "The symbol encoding was created to support fonts with arbitrary ornaments
		// or symbols not supported in Unicode or other standard encodings. A format 4
		// subtable would be used, typically with up to 224 graphic characters
		// assigned at code positions beginning with 0xF020. This corresponds to a
		// sub-range within the Unicode Private-Use Area (PUA), though this is not a
		// Unicode encoding. In legacy usage, some applications would represent the
		// symbol characters in text using a single-byte encoding, and then map 0x20
		// to the OS/2.usFirstCharIndex value in the font. In new fonts, symbols or
		// characters not in Unicode should be encoded using PUA code points in a
		// Unicode 'cmap' subtable."
		return "oh no", nil
	case Encoding{PlatformWindows, EncodingWindowsUnicodeBMP}:
		return decodeUTF16BEBMP(data), nil
	case Encoding{PlatformWindows, EncodingWindowsUnicodeFullRepertoire}:
		return decodeUTF16BE(data), nil
	default:
		return "", UnknownEncodingError{enc}
	}
}
