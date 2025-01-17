// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build ignore

package text

// TODO compare with Android's fonts.xml

import xlanguage "honnef.co/go/gutter/internal/language"

var genres = map[string]FontGenre{
	"ui":        UI,
	"sans":      Sans,
	"serif":     Serif,
	"kai":       Kai,
	"fangsong":  FangSong,
	"monospace": Monospace,
	"emoji":     Emoji,
	"kufic":     Kufic,
	"naskh":     Naskh,
	"nastaliq":  Nastaliq,
	"rashi":     Rashi,
	"math":      Math,
	"hei":       Hei,
	"song":      Song,
	"ming":      Ming,
	"mincho":    Mincho,
}

var notoSansCJK = []string{
	"Noto Sans CJK",
	"Source Han Sans",
	"Noto Sans CJK SC",
	"Noto Sans CJK TC",
	"Noto Sans CJK HK",
	"Noto Sans CJK JP",
	"Noto Sans CJK KR",
}

var notoSerifCJK = []string{
	"Noto Serif CJK",
	"Source Han Serif",
	"Noto Serif CJK HK",
	"Noto Serif CJK JP",
	"Noto Serif CJK KR",
	"Noto Serif CJK SC",
	"Noto Serif CJK TC",
}

var notoSansMonoCJK = []string{
	"Noto Sans Mono CJK",
	"Noto Sans Mono CJK HK",
	"Noto Sans Mono CJK JP",
	"Noto Sans Mono CJK KR",
	"Noto Sans Mono CJK SC",
	"Noto Sans Mono CJK TC",
}

var IBMPlexFonts = FontFamilies{
	{xlanguage.Und, Monospace}: {"IBM Plex Mono"},

	{xlanguage.Und, UI}:    {"default:und/sans"},
	{xlanguage.Und, Sans}:  {"IBM Plex Sans"},
	{xlanguage.Und, Serif}: {"IBM Plex Serif"},

	{Tag("und-Latn"), Sans}: {"default:und/sans"},
	{Tag("und-Grek"), Sans}: {"default:und/sans"},
	{Tag("und-Cyrl"), Sans}: {"default:und/sans"},

	{Tag("und-Arab"), UI}:   {"default:und-Arab/sans"},
	{Tag("und-Arab"), Sans}: {"IBM Plex Sans Arabic"},

	{Tag("und-Deva"), Sans}: {"IBM Plex Sans Devanagari"},
	{Tag("und-Hebr"), Sans}: {"IBM Plex Sans Hebrew"},
	{Tag("und-Thai"), Sans}: {"Noto Sans Thai"},

	{Tag("und-Hira"), Sans}: {"default:und-Hrkt/sans"},
	{Tag("und-Kana"), Sans}: {"default:und/Hrkt/sans"},
	{Tag("und-Hrkt"), Sans}: {"IBM Plex Sans JP"},
	{Tag("und-Hang"), Sans}: {"IBM Plex Sans KR"},
	{Tag("ja-Hani"), Sans}:  {"IBM Plex Sans JP"},
}

var NotoFonts = FontFamilies{
	// Assorted symbol fonts to try
	{xlanguage.Und, NoGenre}: {
		// Noto Sans might contain useful symbols, too
		"Noto Sans",
		"Noto Sans Math",
		"Noto Sans Symbols",
		"Noto Sans Symbols 2",
		"Noto Serif Ottoman Siyaq",
		"Noto Music",
	},

	{xlanguage.Und, UI}:   {"default:und/sans"},
	{xlanguage.Und, Sans}: {"Noto Sans"},
	// {xlanguage.Und, Sans}:      {"Sans Bullshit Sans"},
	{xlanguage.Und, Serif}:     {"Noto Serif"},
	{xlanguage.Und, Monospace}: {"Noto Sans Mono"},
	{xlanguage.Und, Emoji}:     {"Noto Color Emoji"},
	{xlanguage.Und, Math}: {
		// Use the math font as the primary, in case it has better shapes for greek and
		// latin letters.
		"Noto Sans Math",
		// Fallbacks for things not covered by the math font
		"default:und-Latn/sans",
		"default:und-Grek/sans",
		"default:und/sans",
		"Noto Sans Symbols",
		"Noto Sans Symbols 2",
	},

	// We don't have to specify these, as the catch-all is the same, but it documents some
	// of the common scripts one might want to specify different fonts for.
	{Tag("und-Latn"), Sans}:  {"default:und/sans"},
	{Tag("und-Grek"), Sans}:  {"default:und/sans"},
	{Tag("und-Cyrl"), Sans}:  {"default:und/sans"},
	{Tag("und-Latn"), Serif}: {"default:und/serif"},
	{Tag("und-Grek"), Serif}: {"default:und/serif"},
	{Tag("und-Cyrl"), Serif}: {"default:und/serif"},

	{Tag("und-Armn"), Sans}:  {"Noto Sans Armenian"},
	{Tag("und-Armn"), Serif}: {"Noto Serif Armenian"},

	{Tag("und-Thai"), Sans}:  {"Noto Sans Thai Looped"},
	{Tag("und-Thai"), Serif}: {"Noto Serif Thai"},

	// Prefer Noto Sans CJK as it covers all regions and supports selecting the
	// correct region based on region tags. Try the other Noto Sans CJK fonts in
	// case only those are installed.

	// Hiragana
	{Tag("und-Hira"), Sans}: {"default:und-Hrkt/sans"},
	// Katakana
	{Tag("und-Kana"), Sans}: {"default:und-Hrkt/sans"},
	// Hiragana + Katakana
	{Tag("und-Hrkt"), Sans}: notoSansCJK,
	// Hanzi, Kanji, Hanja
	{Tag("und-Hani"), Sans}: notoSansCJK,
	// Hangul
	{Tag("und-Hang"), Sans}: notoSansCJK,
	// Kanji; prefer the japanese region font if the generic one isn't available
	{Tag("ja-Hani"), Sans}: {
		"Noto Sans CJK",
		"Noto Sans CJK JP",
		"Noto Sans CJK SC",
		"Noto Sans CJK TC",
		"Noto Sans CJK HK",
		"Noto Sans CJK KR",
	},

	{Tag("und-Hira"), Serif}: {"default:und-Hrkt/serif"},
	{Tag("und-Kana"), Serif}: {"default:und/Hrkt/serif"},
	{Tag("und-Hrkt"), Serif}: notoSerifCJK,
	{Tag("und-Hani"), Serif}: notoSerifCJK,
	{Tag("und-Hang"), Serif}: notoSerifCJK,
	{Tag("ja-Hani"), Serif}: {
		"Noto Serif CJK",
		"Noto Serif CJK JP",
		"Noto Serif CJK SC",
		"Noto Serif CJK TC",
		"Noto Serif CJK HK",
		"Noto Serif CJK KR",
	},

	{Tag("und-Hira"), Monospace}: {"default:und-Hrkt/monospace"},
	{Tag("und-Kana"), Monospace}: {"default:und-Hrkt/monospace"},
	{Tag("und-Hrkt"), Monospace}: notoSansMonoCJK,
	{Tag("und-Hani"), Monospace}: notoSansMonoCJK,
	{Tag("und-Hang"), Monospace}: notoSansMonoCJK,
	{Tag("ja-Hani"), Monospace}: {
		"Noto Sans Mono CJK",
		"Noto Sans Mono CJK JP",
		"Noto Sans Mono CJK SC",
		"Noto Sans Mono CJK TC",
		"Noto Sans Mono CJK HK",
		"Noto Sans Mono CJK KR",
	},

	// These rules aren't strictly necessary because the Noto CJK fonts do the right thing
	// based on language tags, but it's useful to document the different kinds of uses of
	// han one may want to use different fonts for. We may also decide to support more
	// region-specific fonts.
	{Tag("zh-Hani-HK"), Sans}: notoSansCJK, // Chinese (Traditional, Hong Kong)
	// TODO: vietnamese use of han
	{Tag("zh-Hani-MO"), Sans}: notoSansCJK, // Chinese (Traditional, Macao)
	{Tag("zh-Hani-TW"), Sans}: notoSansCJK, // Chinese (Traditional, Taiwan)
	{Tag("zh-Hani-SG"), Sans}: notoSansCJK, // Chinese (Simplified, Singapore)
	{Tag("zh-Hani-CN"), Sans}: notoSansCJK, // Chinese (Simplified, Mainland)

	{Tag("zh-Hani-HK"), Serif}: notoSerifCJK, // Chinese (Traditional, Hong Kong)
	// TODO: vietnamese use of han
	{Tag("zh-Hani-MO"), Serif}: notoSerifCJK, // Chinese (Traditional, Macao)
	{Tag("zh-Hani-TW"), Serif}: notoSerifCJK, // Chinese (Traditional, Taiwan)
	{Tag("zh-Hani-SG"), Serif}: notoSerifCJK, // Chinese (Simplified, Singapore)
	{Tag("zh-Hani-CN"), Serif}: notoSerifCJK, // Chinese (Simplified, Mainland)

	// Most languages using the Arabic script use Naskh and are served by the und-Arab
	// fallback. We only have to specify languages like Urdu (and derivatives) that
	// default to Nastaliq.
	{Tag("und-Arab"), UI}:       {"Noto Naskh Arabic UI"},
	{Tag("und-Arab"), Sans}:     {"Noto Sans Arabic"},
	{Tag("und-Arab"), Naskh}:    {"Noto Naskh Arabic"},
	{Tag("und-Arab"), Serif}:    {"default:und-Arab/naskh"},
	{Tag("und-Arab"), Kufic}:    {"Noto Kufi Arabic"},
	{Tag("und-Arab"), Nastaliq}: {"Noto Nastaliq Urdu"},

	// TODO: should UI really use Nastaliq instead of Naskh UI?
	{Tag("ur-Arab"), UI}:       {"default:ur/nastaliq"},
	{Tag("ur-Arab"), Nastaliq}: {"default:und-Arab/nastaliq"},

	// Khowar is a modification of the Urdu alphabet. Noto doesn't have a dedicated font
	// for it, so we reuse the Urdu one.
	{Tag("khw-Arab"), NoGenre}: {"default:ur"},

	// Burushaski uses a Urdu-derived alphabet with some additional letters. As far as I
	// can tell, Noto Nastaliq Urdu includes those letters.
	{Tag("bsk-Arab"), NoGenre}: {"default:ur"},

	// Balti
	{Tag("bft-Arab"), NoGenre}: {"default:ur"},
	{Tag("bft-Tibt"), UI}:      {"default:bft-Tibt/serif"},
	{Tag("bft-Tibt"), Serif}:   {"Noto Serif Tibetan"},

	// Punjabi using Shahmukhi
	{Tag("pa-Arab"), NoGenre}: {"default:ur"},

	// Lahnda, the macrolanguage containing, among others, Saraiki
	{Tag("lah-Arab"), NoGenre}: {"default:ur"},

	{Tag("und-Hebr"), Sans}:  {"Noto Sans Hebrew"},
	{Tag("und-Hebr"), Serif}: {"Noto Serif Hebrew"},
	{Tag("und-Hebr"), Rashi}: {"Noto Rashi Hebrew"},

	{Tag("und-Yiii"), Sans}:  {"Noto Sans Yi"},
	{Tag("und-Goth"), Sans}:  {"Noto Sans Gothic"},
	{Tag("und-Xsux"), Sans}:  {"Noto Sans Cuneiform"},
	{Tag("und-Avst"), Sans}:  {"Noto Sans Avestan"},
	{Tag("und-Bali"), Sans}:  {"Noto Sans Balinese"},
	{Tag("und-Bali"), Serif}: {"Noto Serif Balinese"},
	{Tag("und-Bamu"), Sans}:  {"Noto Sans Bamum"},
	{Tag("und-Bass"), Sans}:  {"Noto Sans Bassa Vah"},
	{Tag("und-Batk"), Sans}:  {"Noto Sans Batak"},
	{Tag("und-Beng"), Sans}:  {"Noto Sans Bengali"},
	{Tag("und-Beng"), Serif}: {"Noto Serif Bengali"},
	{Tag("und-Bhks"), Sans}:  {"Noto Sans Bhaiksuki"},
	{Tag("und-Brah"), Sans}:  {"Noto Sans Brahmi"},
	{Tag("und-Bugi"), Sans}:  {"Noto Sans Buginese"},
	{Tag("und-Buhd"), Sans}:  {"Noto Sans Buhid"},
	// Meroitic_Cursive
	// Meroitic_Hieroglyphs
	//  Noto Sans Meroitic
	{Tag("und-Modi"), Sans}:  {"Noto Sans Modi"},
	{Tag("und-Mong"), Sans}:  {"Noto Sans Mongolian"},
	{Tag("und-Mroo"), Sans}:  {"Noto Sans Mro"},
	{Tag("und-Mult"), Sans}:  {"Noto Sans Multani"},
	{Tag("und-Mymr"), Sans}:  {"Noto Sans Myanmar"},
	{Tag("und-Mymr"), Serif}: {"Noto Serif Myanmar"},
	{Tag("und-Nagm"), Sans}:  {"Noto Sans Nag Mundari"},
	{Tag("und-Nand"), Sans}:  {"Noto Sans Nandinagari"},
	{Tag("und-Nbat"), Sans}:  {"Noto Sans Nabataean"},
	{Tag("und-Phlp"), Sans}:  {"Noto Sans Psalter Pahlavi"},
	{Tag("und-Phnx"), Sans}:  {"Noto Sans Phoenician"},
	{Tag("und-Plrd"), Sans}:  {"Noto Sans Miao"},
	{Tag("und-Rjng"), Sans}:  {"Noto Sans Rejang"},
	{Tag("und-Runr"), Sans}:  {"Noto Sans Runic"},
	{Tag("und-Samr"), Sans}:  {"Noto Sans Samaritan"},
	{Tag("und-Saur"), Sans}:  {"Noto Sans Saurashtra"},
	{Tag("und-Sgnw"), Sans}:  {"Noto Sans SignWriting"},
	{Tag("und-Shaw"), Sans}:  {"Noto Sans Shavian"},
	{Tag("und-Shrd"), Sans}:  {"Noto Sans Sharada"},
	{Tag("und-Sidd"), Sans}:  {"Noto Sans Siddham"},
	{Tag("und-Sinh"), Sans}:  {"Noto Sans Sinhala"},
	{Tag("und-Sinh"), Serif}: {"Noto Serif Sinhala"},
	{Tag("und-Sogd"), Sans}:  {"Noto Sans Sogdian"},
	{Tag("und-Sora"), Sans}:  {"Noto Sans Sora Sompeng"},
	{Tag("und-Soyo"), Sans}:  {"Noto Sans Soyombo"},
	{Tag("und-Sund"), Sans}:  {"Noto Sans Sundanese"},
	{Tag("und-Sylo"), Sans}:  {"Noto Sans Syloti Nagri"},
	{Tag("und-Cakm"), Sans}:  {"Noto Sans Chakma"},
	{Tag("und-Cham"), Sans}:  {"Noto Sans Cham"},
	{Tag("und-Cher"), Sans}:  {"Noto Sans Cherokee"},
	{Tag("und-Chrs"), Sans}:  {"Noto Sans Chorasmian"},
	{Tag("und-Copt"), Sans}:  {"Noto Sans Coptic"},
	{Tag("und-Cprt"), Sans}:  {"Noto Sans Cypriot"},
	{Tag("und-Cpmn"), Sans}:  {"Noto Sans Cypro Minoan"},
	{Tag("und-Dsrt"), Sans}:  {"Noto Sans Deseret"},
	{Tag("und-Deva"), Sans}:  {"Noto Sans Devanagari"},
	{Tag("und-Deva"), Serif}: {"Noto Serif Devanagari"},
	{Tag("und-Dupl"), Sans}:  {"Noto Sans Duployan"},
	{Tag("und-Egyp"), Sans}:  {"Noto Sans Egyptian Hieroglyphs"},
	{Tag("und-Elba"), Sans}:  {"Noto Sans Elbasan"},
	{Tag("und-Elym"), Sans}:  {"Noto Sans Elymaic"},
	{Tag("und-Ethi"), Sans}:  {"Noto Sans Ethiopic"},
	{Tag("und-Ethi"), Serif}: {"Noto Serif Ethiopic"},
	{Tag("und-Geor"), Sans}:  {"Noto Sans Georgian"},
	{Tag("und-Geor"), Serif}: {"Noto Serif Georgian"},
	{Tag("und-Glag"), Sans}:  {"Noto Sans Glagolitic"},
	{Tag("und-Gran"), Sans}:  {"Noto Sans Grantha"},
	{Tag("und-Gran"), Serif}: {"Noto Serif Grantha"},
	{Tag("und-Gujr"), Sans}:  {"Noto Sans Gujarati"},
	{Tag("und-Gujr"), Serif}: {"Noto Serif Gujarati"},
	{Tag("und-Gong"), Sans}:  {"Noto Sans Gunjala Gondi"},
	{Tag("und-Guru"), Sans}:  {"Noto Sans Gurmukhi"},
	{Tag("und-Guru"), Serif}: {"Noto Serif Gurmukhi"},
	{Tag("und-Rohg"), Sans}:  {"Noto Sans Hanifi Rohingya"},
	{Tag("und-Hano"), Sans}:  {"Noto Sans Hanunoo"},
	{Tag("und-Hatr"), Sans}:  {"Noto Sans Hatran"},
	{Tag("und-Lisu"), Sans}:  {"Noto Sans Lisu"},
	{Tag("und-Lyci"), Sans}:  {"Noto Sans Lycian"},
	{Tag("und-Lydi"), Sans}:  {"Noto Sans Lydian"},
	{Tag("und-Mahj"), Sans}:  {"Noto Sans Mahajani"},
	{Tag("und-Mlym"), Sans}:  {"Noto Sans Malayalam"},
	{Tag("und-Mlym"), Serif}: {"Noto Serif Malayalam"},
	{Tag("und-Mand"), Sans}:  {"Noto Sans Mandaic"},
	{Tag("und-Mani"), Sans}:  {"Noto Sans Manichaean"},
	{Tag("und-Marc"), Sans}:  {"Noto Sans Marchen"},
	{Tag("und-Gonm"), Sans}:  {"Noto Sans Masaram Gondi"},
	{Tag("und-Maya"), Sans}:  {"Noto Sans Mayan Numerals"},
	{Tag("und-Medf"), Sans}:  {"Noto Sans Medefaidrin"},
	{Tag("und-Mtei"), Sans}:  {"Noto Sans Meetei Mayek"},
	{Tag("und-Mend"), Sans}:  {"Noto Sans Mende Kikakui"},
	{Tag("und-Newa"), Sans}:  {"Noto Sans Newa"},
	{Tag("und-Orya"), Sans}:  {"Noto Sans Oriya"},
	{Tag("und-Orya"), Serif}: {"Noto Serif Oriya"},
	{Tag("und-Osge"), Sans}:  {"Noto Sans Osage"},
	{Tag("und-Osma"), Sans}:  {"Noto Sans Osmanya"},
	{Tag("und-Hmng"), Sans}:  {"Noto Sans Pahawh Hmong"},
	{Tag("und-Palm"), Sans}:  {"Noto Sans Palmyrene"},
	{Tag("und-Pauc"), Sans}:  {"Noto Sans Pau Cin Hau"},
	{Tag("und-Tnsa"), Sans}:  {"Noto Sans Tangsa"},
	{Tag("und-Telu"), Sans}:  {"Noto Sans Telugu"},
	{Tag("und-Telu"), Serif}: {"Noto Serif Telugu"},
	{Tag("und-Thaa"), Sans}:  {"Noto Sans Thaana"},
	{Tag("und-Cari"), Sans}:  {"Noto Sans Carian"},
	{Tag("und-Java"), Sans}:  {"Noto Sans Javanese"},
	{Tag("und-Kthi"), Sans}:  {"Noto Sans Kaithi"},
	{Tag("und-Knda"), Sans}:  {"Noto Sans Kannada"},
	{Tag("und-Knda"), Serif}: {"Noto Serif Kannada"},
	{Tag("und-Kawi"), Sans}:  {"Noto Sans Kawi"},
	{Tag("und-Kali"), Sans}:  {"Noto Sans Kayah Li"},
	{Tag("und-Khar"), Sans}:  {"Noto Sans Kharoshthi"},
	{Tag("und-Khmr"), Sans}:  {"Noto Sans Khmer"},
	{Tag("und-Khmr"), Serif}: {"Noto Serif Khmer"},
	{Tag("und-Khoj"), Sans}:  {"Noto Sans Khojki"},
	{Tag("und-Khoj"), Serif}: {"Noto Serif Khojki"},
	{Tag("und-Sind"), Sans}:  {"Noto Sans Khudawadi"},
	{Tag("und-Lepc"), Sans}:  {"Noto Sans Lepcha"},
	{Tag("und-Limb"), Sans}:  {"Noto Sans Limbu"},
	{Tag("und-Lina"), Sans}:  {"Noto Sans Linear A"},
	{Tag("und-Linb"), Sans}:  {"Noto Sans Linear B"},
	{Tag("und-Talu"), Sans}:  {"Noto Sans New Tai Lue"},
	{Tag("und-Tirh"), Sans}:  {"Noto Sans Tirhuta"},
	{Tag("und-Ugar"), Sans}:  {"Noto Sans Ugaritic"},
	{Tag("und-Vaii"), Sans}:  {"Noto Sans Vai"},
	{Tag("und-Vith"), Sans}:  {"Noto Sans Vithkuqi"},
	{Tag("und-Vith"), Serif}: {"Noto Serif Vithkuqi"},
	{Tag("und-Wcho"), Sans}:  {"Noto Sans Wancho"},
	{Tag("und-Wara"), Sans}:  {"Noto Sans Warang Citi"},
	{Tag("und-Zanb"), Sans}:  {"Noto Sans Zanabazar Square"},
	{Tag("und-Hluw"), Sans}:  {"Noto Sans Anatolian Hieroglyphs"},
	{Tag("und-Cans"), Sans}:  {"Noto Sans Canadian Aboriginal"},
	{Tag("und-Aghb"), Sans}:  {"Noto Sans Caucasian Albanian"},
	{Tag("und-Armi"), Sans}:  {"Noto Sans Imperial Aramaic"},
	{Tag("und-Nshu"), Sans}:  {"Noto Sans Nushu"},
	{Tag("und-Ogam"), Sans}:  {"Noto Sans Ogham"},
	{Tag("und-Olck"), Sans}:  {"Noto Sans Ol Chiki"},
	{Tag("und-Hung"), Sans}:  {"Noto Sans Old Hungarian"},
	{Tag("und-Ital"), Sans}:  {"Noto Sans Old Italic"},
	{Tag("und-Narb"), Sans}:  {"Noto Sans Old North Arabian"},
	{Tag("und-Perm"), Sans}:  {"Noto Sans Old Permic"},
	{Tag("und-Xpeo"), Sans}:  {"Noto Sans Old Persian"},
	{Tag("und-Sogo"), Sans}:  {"Noto Sans Old Sogdian"},
	{Tag("und-Sarb"), Sans}:  {"Noto Sans Old South Arabian"},
	{Tag("und-Orkh"), Sans}:  {"Noto Sans Old Turkic"},
	{Tag("und-Ougr"), Serif}: {"Noto Serif Old Uyghur"},
	{Tag("und-Yezi"), Serif}: {"Noto Serif Yezidi"},
	{Tag("und-Syrc"), Sans}:  {"Noto Sans Syriac"},
	{Tag("und-Syre"), Sans}:  {"Noto Sans Syriac"},
	{Tag("und-Syrj"), Sans}:  {"Noto Sans Syriac Western"},
	{Tag("und-Syrn"), Sans}:  {"Noto Sans Syriac Eastern"},
	{Tag("und-Tglg"), Sans}:  {"Noto Sans Tagalog"},
	{Tag("und-Tagb"), Sans}:  {"Noto Sans Tagbanwa"},
	{Tag("und-Tale"), Sans}:  {"Noto Sans Tai Le"},
	{Tag("und-Lana"), Sans}:  {"Noto Sans Tai Tham"},
	{Tag("und-Tavt"), Sans}:  {"Noto Sans Tai Viet"},
	{Tag("und-Takr"), Sans}:  {"Noto Sans Takri"},
	{Tag("und-Ahom"), Serif}: {"Noto Serif Ahom"},
	{Tag("und-Diak"), Serif}: {"Noto Serif Dives Akuru"},
	{Tag("und-Dogr"), Serif}: {"Noto Serif Dogra"},
	{Tag("und-Kits"), Serif}: {"Noto Serif Khitan Small Script"},
	{Tag("und-Maka"), Serif}: {"Noto Serif Makasar"},
	{Tag("und-Hmnp"), Serif}: {"Noto Serif NP Hmong"},
	{Tag("und-Tang"), Serif}: {"Noto Serif Tangut"},
	{Tag("und-Tibt"), Serif}: {"Noto Serif Tibetan"},
	{Tag("und-Toto"), Serif}: {"Noto Serif Toto"},
	{Tag("und-Adlm"), Sans}:  {"Noto Sans Adlam"},
	{Tag("und-Taml"), Sans}:  {"Noto Sans Tamil", "Noto Sans Tamil Supplement"},
	{Tag("und-Taml"), Serif}: {"Noto Serif Tamil", "Noto Sans Tamil Supplement"},
}
