// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

import "golang.org/x/text/language"

// Encoding IDs for PlatformMacintosh
const (
	EncodingMacintoshRoman              EncodingID = 0
	EncodingMacintoshJapanese           EncodingID = 1
	EncodingMacintoshTraditionalChinese EncodingID = 2
	EncodingMacintoshKorean             EncodingID = 3
	EncodingMacintoshArabic             EncodingID = 4
	EncodingMacintoshHebrew             EncodingID = 5
	EncodingMacintoshGreek              EncodingID = 6
	EncodingMacintoshCyrillic           EncodingID = 7
	EncodingMacintoshRSymbol            EncodingID = 8
	EncodingMacintoshDevanagari         EncodingID = 9
	EncodingMacintoshGurmukhi           EncodingID = 10
	EncodingMacintoshGujarati           EncodingID = 11
	EncodingMacintoshOdia               EncodingID = 12
	EncodingMacintoshBangla             EncodingID = 13
	EncodingMacintoshTamil              EncodingID = 14
	EncodingMacintoshTelugu             EncodingID = 15
	EncodingMacintoshKannada            EncodingID = 16
	EncodingMacintoshMalayalam          EncodingID = 17
	EncodingMacintoshSinhalese          EncodingID = 18
	EncodingMacintoshBurmese            EncodingID = 19
	EncodingMacintoshKhmer              EncodingID = 20
	EncodingMacintoshThai               EncodingID = 21
	EncodingMacintoshLaotian            EncodingID = 22
	EncodingMacintoshGeorgian           EncodingID = 23
	EncodingMacintoshArmenian           EncodingID = 24
	EncodingMacintoshSimplifiedChinese  EncodingID = 25
	EncodingMacintoshTibetan            EncodingID = 26
	EncodingMacintoshMongolian          EncodingID = 27
	EncodingMacintoshGeez               EncodingID = 28
	EncodingMacintoshSlavic             EncodingID = 29
	EncodingMacintoshVietnamese         EncodingID = 30
	EncodingMacintoshSindhi             EncodingID = 31
	EncodingMacintoshUninterpreted      EncodingID = 32
)

// Macintosh language IDs
const (
	LanguageMacintoshEnglish             = 0
	LanguageMacintoshFrench              = 1
	LanguageMacintoshGerman              = 2
	LanguageMacintoshItalian             = 3
	LanguageMacintoshDutch               = 4
	LanguageMacintoshSwedish             = 5
	LanguageMacintoshSpanish             = 6
	LanguageMacintoshDanish              = 7
	LanguageMacintoshPortuguese          = 8
	LanguageMacintoshNorwegian           = 9
	LanguageMacintoshHebrew              = 10
	LanguageMacintoshJapanese            = 11
	LanguageMacintoshArabic              = 12
	LanguageMacintoshFinnish             = 13
	LanguageMacintoshModernGreek         = 14
	LanguageMacintoshIcelandic           = 15
	LanguageMacintoshMaltese             = 16
	LanguageMacintoshTurkish             = 17
	LanguageMacintoshCroatian            = 18
	LanguageMacintoshTraditionalChinese  = 19
	LanguageMacintoshUrdu                = 20
	LanguageMacintoshHindi               = 21
	LanguageMacintoshThai                = 22
	LanguageMacintoshKorean              = 23
	LanguageMacintoshLithuanian          = 24
	LanguageMacintoshPolish              = 25
	LanguageMacintoshHungarian           = 26
	LanguageMacintoshEstonian            = 27
	LanguageMacintoshLatvian             = 28
	LanguageMacintoshNorthernSami        = 29
	LanguageMacintoshFaroese             = 30
	LanguageMacintoshPersian             = 31
	LanguageMacintoshRussian             = 32
	LanguageMacintoshSimplifiedChinese   = 33
	LanguageMacintoshBelgian             = 34
	LanguageMacintoshIrish               = 35
	LanguageMacintoshAlbanian            = 36
	LanguageMacintoshRomanian            = 37
	LanguageMacintoshCzech               = 38
	LanguageMacintoshSlovak              = 39
	LanguageMacintoshSlovenian           = 40
	LanguageMacintoshYiddish             = 41
	LanguageMacintoshSerbian             = 42
	LanguageMacintoshMacedonian          = 43
	LanguageMacintoshBulgarian           = 44
	LanguageMacintoshUkrainian           = 45
	LanguageMacintoshBelarusian          = 46
	LanguageMacintoshUzbek               = 47
	LanguageMacintoshKazakh              = 48
	LanguageMacintoshAzerbaijaniCyrillic = 49
	LanguageMacintoshAzerbaijaniArabic   = 50
	LanguageMacintoshArmenian            = 51
	LanguageMacintoshGeorgian            = 52
	LanguageMacintoshMoldovan            = 53
	LanguageMacintoshKirghiz             = 54
	LanguageMacintoshTajik               = 55
	LanguageMacintoshTurkmen             = 56
	LanguageMacintoshMongolian           = 57
	LanguageMacintoshMongolianCyrillic   = 58
	LanguageMacintoshPashto              = 59
	LanguageMacintoshKurdish             = 60
	LanguageMacintoshKashmiri            = 61
	LanguageMacintoshSindhi              = 62
	LanguageMacintoshTibetan             = 63
	LanguageMacintoshNepali              = 64
	LanguageMacintoshSanskrit            = 65
	LanguageMacintoshMarathi             = 66
	LanguageMacintoshBengali             = 67
	LanguageMacintoshAssamese            = 68
	LanguageMacintoshGujarati            = 69
	LanguageMacintoshPunjabi             = 70
	LanguageMacintoshOdia                = 71
	LanguageMacintoshMalayalam           = 72
	LanguageMacintoshKannada             = 73
	LanguageMacintoshTamil               = 74
	LanguageMacintoshTelugu              = 75
	LanguageMacintoshSinhala             = 76
	LanguageMacintoshBurmese             = 77
	LanguageMacintoshKhmer               = 78
	LanguageMacintoshLao                 = 79
	LanguageMacintoshVietnamese          = 80
	LanguageMacintoshIndonesian          = 81
	LanguageMacintoshTagalog             = 82
	LanguageMacintoshMalayLatin          = 83
	LanguageMacintoshMalayArabic         = 84
	LanguageMacintoshAmharic             = 85
	LanguageMacintoshTigrinya            = 86
	LanguageMacintoshOromo               = 87
	LanguageMacintoshSomali              = 88
	LanguageMacintoshSwahili             = 89
	LanguageMacintoshKinyarwanda         = 90
	LanguageMacintoshRundi               = 91
	LanguageMacintoshNyanja              = 92
	LanguageMacintoshMalagasy            = 93
	LanguageMacintoshEsperanto           = 94
	LanguageMacintoshWelsh               = 128
	LanguageMacintoshBasque              = 129
	LanguageMacintoshCatalan             = 130
	LanguageMacintoshLatin               = 131
	LanguageMacintoshQuechua             = 132
	LanguageMacintoshGuarani             = 133
	LanguageMacintoshAymara              = 134
	LanguageMacintoshTatar               = 135
	LanguageMacintoshUyghur              = 136
	LanguageMacintoshDzongkha            = 137
	LanguageMacintoshJavaneseLatin       = 138
	LanguageMacintoshSundaneseLatin      = 139
	LanguageMacintoshGalician            = 140
	LanguageMacintoshmacAfrikaans        = 141
	LanguageMacintoshBreton              = 142
	LanguageMacintoshInuktitut           = 143
	LanguageMacintoshGaelic              = 144
	LanguageMacintoshManx                = 145
	LanguageMacintoshIrishGaelic         = 146
	LanguageMacintoshTonga               = 147
	LanguageMacintoshAncientGreek        = 148
	LanguageMacintoshGreenlandic         = 149
	LanguageMacintoshAzerbaijaniLatin    = 150
	LanguageMacintoshNynorsk             = 151
)

// Mapping from Macintosh language IDs to BCP-47 tags. Based on
// https://github.com/apple/swift-corelibs-foundation/blob/77af9c5a984f40e186c0ba127cff88f2a2588c15/CoreFoundation/Locale.subproj/CFLocaleIdentifier.c#L213.
var MacintoshLanguageIDs = [...]language.Tag{
	LanguageMacintoshEnglish:             language.MustParse("en"),
	LanguageMacintoshFrench:              language.MustParse("fr"),
	LanguageMacintoshGerman:              language.MustParse("de"),
	LanguageMacintoshItalian:             language.MustParse("it"),
	LanguageMacintoshDutch:               language.MustParse("nl"),
	LanguageMacintoshSwedish:             language.MustParse("sv"),
	LanguageMacintoshSpanish:             language.MustParse("es"),
	LanguageMacintoshDanish:              language.MustParse("da"),
	LanguageMacintoshPortuguese:          language.MustParse("pt"),
	LanguageMacintoshNorwegian:           language.MustParse("no"),
	LanguageMacintoshHebrew:              language.MustParse("he"),
	LanguageMacintoshJapanese:            language.MustParse("ja"),
	LanguageMacintoshArabic:              language.MustParse("ar"),
	LanguageMacintoshFinnish:             language.MustParse("fi"),
	LanguageMacintoshModernGreek:         language.MustParse("el"),
	LanguageMacintoshIcelandic:           language.MustParse("is"),
	LanguageMacintoshMaltese:             language.MustParse("mt"),
	LanguageMacintoshTurkish:             language.MustParse("tr"),
	LanguageMacintoshCroatian:            language.MustParse("hr"),
	LanguageMacintoshTraditionalChinese:  language.MustParse("zh-Hant"),
	LanguageMacintoshUrdu:                language.MustParse("ur"),
	LanguageMacintoshHindi:               language.MustParse("hi"),
	LanguageMacintoshThai:                language.MustParse("th"),
	LanguageMacintoshKorean:              language.MustParse("ko"),
	LanguageMacintoshLithuanian:          language.MustParse("lt"),
	LanguageMacintoshPolish:              language.MustParse("pl"),
	LanguageMacintoshHungarian:           language.MustParse("hu"),
	LanguageMacintoshEstonian:            language.MustParse("et"),
	LanguageMacintoshLatvian:             language.MustParse("lv"),
	LanguageMacintoshNorthernSami:        language.MustParse("se"),
	LanguageMacintoshFaroese:             language.MustParse("fo"),
	LanguageMacintoshPersian:             language.MustParse("fa"),
	LanguageMacintoshRussian:             language.MustParse("ru"),
	LanguageMacintoshSimplifiedChinese:   language.MustParse("zh-Hans"),
	LanguageMacintoshBelgian:             language.MustParse("nl-BE"),
	LanguageMacintoshIrish:               language.MustParse("ga"),
	LanguageMacintoshAlbanian:            language.MustParse("sq"),
	LanguageMacintoshRomanian:            language.MustParse("ro"),
	LanguageMacintoshCzech:               language.MustParse("cs"),
	LanguageMacintoshSlovak:              language.MustParse("sk"),
	LanguageMacintoshSlovenian:           language.MustParse("sl"),
	LanguageMacintoshYiddish:             language.MustParse("yi"),
	LanguageMacintoshSerbian:             language.MustParse("sr"),
	LanguageMacintoshMacedonian:          language.MustParse("mk"),
	LanguageMacintoshBulgarian:           language.MustParse("bg"),
	LanguageMacintoshUkrainian:           language.MustParse("uk"),
	LanguageMacintoshBelarusian:          language.MustParse("be"),
	LanguageMacintoshUzbek:               language.MustParse("uz"),
	LanguageMacintoshKazakh:              language.MustParse("kk"),
	LanguageMacintoshAzerbaijaniCyrillic: language.MustParse("az-Cyrl"),
	LanguageMacintoshAzerbaijaniArabic:   language.MustParse("az-Arab"),
	LanguageMacintoshArmenian:            language.MustParse("hy"),
	LanguageMacintoshGeorgian:            language.MustParse("ka"),
	LanguageMacintoshMoldovan:            language.MustParse("mo"),
	LanguageMacintoshKirghiz:             language.MustParse("ky"),
	LanguageMacintoshTajik:               language.MustParse("tg"),
	LanguageMacintoshTurkmen:             language.MustParse("tk-Cyrl"),
	LanguageMacintoshMongolian:           language.MustParse("mn-Mong"),
	LanguageMacintoshMongolianCyrillic:   language.MustParse("mn-Cyrl"),
	LanguageMacintoshPashto:              language.MustParse("ps"),
	LanguageMacintoshKurdish:             language.MustParse("ku"),
	LanguageMacintoshKashmiri:            language.MustParse("ks"),
	LanguageMacintoshSindhi:              language.MustParse("sd"),
	LanguageMacintoshTibetan:             language.MustParse("bo"),
	LanguageMacintoshNepali:              language.MustParse("ne"),
	LanguageMacintoshSanskrit:            language.MustParse("sa"),
	LanguageMacintoshMarathi:             language.MustParse("mr"),
	LanguageMacintoshBengali:             language.MustParse("bn"),
	LanguageMacintoshAssamese:            language.MustParse("as"),
	LanguageMacintoshGujarati:            language.MustParse("gu"),
	LanguageMacintoshPunjabi:             language.MustParse("pa"),
	LanguageMacintoshOdia:                language.MustParse("or"),
	LanguageMacintoshMalayalam:           language.MustParse("ml"),
	LanguageMacintoshKannada:             language.MustParse("kn"),
	LanguageMacintoshTamil:               language.MustParse("ta"),
	LanguageMacintoshTelugu:              language.MustParse("te"),
	LanguageMacintoshSinhala:             language.MustParse("si"),
	LanguageMacintoshBurmese:             language.MustParse("my"),
	LanguageMacintoshKhmer:               language.MustParse("km"),
	LanguageMacintoshLao:                 language.MustParse("lo"),
	LanguageMacintoshVietnamese:          language.MustParse("vi"),
	LanguageMacintoshIndonesian:          language.MustParse("id"),
	LanguageMacintoshTagalog:             language.MustParse("tl"),
	LanguageMacintoshMalayLatin:          language.MustParse("ms"),
	LanguageMacintoshMalayArabic:         language.MustParse("ms-Arab"),
	LanguageMacintoshAmharic:             language.MustParse("am"),
	LanguageMacintoshTigrinya:            language.MustParse("ti"),
	LanguageMacintoshOromo:               language.MustParse("om"),
	LanguageMacintoshSomali:              language.MustParse("so"),
	LanguageMacintoshSwahili:             language.MustParse("sw"),
	LanguageMacintoshKinyarwanda:         language.MustParse("rw"),
	LanguageMacintoshRundi:               language.MustParse("rn"),
	LanguageMacintoshNyanja:              language.MustParse("ny"),
	LanguageMacintoshMalagasy:            language.MustParse("mg"),
	LanguageMacintoshEsperanto:           language.MustParse("eo"),
	LanguageMacintoshWelsh:               language.MustParse("cy"),
	LanguageMacintoshBasque:              language.MustParse("eu"),
	LanguageMacintoshCatalan:             language.MustParse("ca"),
	LanguageMacintoshLatin:               language.MustParse("la"),
	LanguageMacintoshQuechua:             language.MustParse("qu"),
	LanguageMacintoshGuarani:             language.MustParse("gn"),
	LanguageMacintoshAymara:              language.MustParse("ay"),
	LanguageMacintoshTatar:               language.MustParse("tt-Cyrl"),
	LanguageMacintoshUyghur:              language.MustParse("ug"),
	LanguageMacintoshDzongkha:            language.MustParse("dz"),
	LanguageMacintoshJavaneseLatin:       language.MustParse("jv-Latn"),
	LanguageMacintoshSundaneseLatin:      language.MustParse("su-Latn"),
	LanguageMacintoshGalician:            language.MustParse("gl"),
	LanguageMacintoshmacAfrikaans:        language.MustParse("af"),
	LanguageMacintoshBreton:              language.MustParse("br"),
	LanguageMacintoshInuktitut:           language.MustParse("iu"),
	LanguageMacintoshGaelic:              language.MustParse("gd"),
	LanguageMacintoshManx:                language.MustParse("gv"),
	LanguageMacintoshIrishGaelic:         language.MustParse("ga-Latg"),
	LanguageMacintoshTonga:               language.MustParse("to"),
	LanguageMacintoshAncientGreek:        language.MustParse("grc"),
	LanguageMacintoshGreenlandic:         language.MustParse("kl"),
	LanguageMacintoshAzerbaijaniLatin:    language.MustParse("az-Latn"),
	LanguageMacintoshNynorsk:             language.MustParse("nn"),
}
