// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build ignore

package text

import xlanguage "honnef.co/go/gutter/internal/language"

// import xlanguage "golang.org/x/text/language"

var macrolanguages = map[xlanguage.Language]xlanguage.Language{
	// Akan
	base("fat"): base("ak"), // Fanti
	base("twi"): base("ak"), // Twi

	// Arabic
	base("aao"): base("ar"), // Algerian Saharan Arabic
	base("abh"): base("ar"), // Tajiki Arabic
	base("abv"): base("ar"), // Baharna Arabic
	base("acm"): base("ar"), // Mesopotamian Arabic
	base("acq"): base("ar"), // Ta'izzi-Adeni Arabic
	base("acw"): base("ar"), // Hijazi Arabic
	base("acx"): base("ar"), // Omani Arabic
	base("acy"): base("ar"), // Cypriot Arabic
	base("adf"): base("ar"), // Dhofari Arabic
	base("aeb"): base("ar"), // Tunisian Arabic
	base("aec"): base("ar"), // Saidi Arabic
	base("afb"): base("ar"), // Gulf Arabic
	base("apc"): base("ar"), // Levantine Arabic
	base("apd"): base("ar"), // Sudanese Arabic
	base("arb"): base("ar"), // Standard Arabic
	base("arq"): base("ar"), // Algerian Arabic
	base("ars"): base("ar"), // Najdi Arabic
	base("ary"): base("ar"), // Moroccan Arabic
	base("arz"): base("ar"), // Egyptian Arabic
	base("auz"): base("ar"), // Uzbeki Arabic
	base("avl"): base("ar"), // Eastern Egyptian Bedawi Arabic
	base("ayh"): base("ar"), // Hadrami Arabic
	base("ayl"): base("ar"), // Libyan Arabic
	base("ayn"): base("ar"), // Sanaani Arabic
	base("ayp"): base("ar"), // North Mesopotamian Arabic
	base("pga"): base("ar"), // Sudanese Creole Arabic
	base("shu"): base("ar"), // Chadian Arabic
	base("ssh"): base("ar"), // Shihhi Arabic

	// Aymara
	base("ayc"): base("ay"), // Southern Aymara
	base("ayr"): base("ay"), // Central Aymara

	// Azerbaijani
	base("azb"): base("az"), // South Azerbaijani
	base("azj"): base("az"), // North Azerbaijani

	// Baluchi
	base("bcc"): base("bal"), // Southern Balochi
	base("bgn"): base("bal"), // Western Balochi
	base("bgp"): base("bal"), // Eastern Balochi

	// Bikol
	base("bcl"): base("bik"), // Central Bikol
	base("bln"): base("bik"), // Southern Catanduanes Bikol
	base("bto"): base("bik"), // Rinconada Bikol
	base("cts"): base("bik"), // Northern Catanduanes Bikol
	base("fbl"): base("bik"), // West Albay Bikol
	base("lbl"): base("bik"), // Libon Bikol
	base("rbl"): base("bik"), // Miraya Bikol
	base("ubl"): base("bik"), // Buhi'non Bikol

	// Bontok
	base("ebk"): base("bnc"), // Eastern Bontok
	base("lbk"): base("bnc"), // Central Bontok
	base("obk"): base("bnc"), // Southern Bontok
	base("rbk"): base("bnc"), // Northern Bontok
	base("vbk"): base("bnc"), // Southwestern Bontok

	// Buriat
	base("bxm"): base("bua"), // Mongolia Buriat
	base("bxr"): base("bua"), // Russia Buriat
	base("bxu"): base("bua"), // China Buriat

	// Mari
	base("mhr"): base("chm"), // Eastern Mari
	base("mrj"): base("chm"), // Western Mari

	// Cree
	base("crj"): base("cr"), // Southern East Cree
	base("crk"): base("cr"), // Plains Cree
	base("crl"): base("cr"), // Northern East Cree
	base("crm"): base("cr"), // Moose Cree
	base("csw"): base("cr"), // Swampy Cree
	base("cwd"): base("cr"), // Woods Cree

	// Delaware
	base("umu"): base("del"), // Munsee
	base("unm"): base("del"), // Unami

	// Slave
	base("scs"): base("den"), // North Slavey
	base("xsl"): base("den"), // South Slavey

	// Dinka
	base("dib"): base("din"), // South Central Dinka
	base("dik"): base("din"), // Southwestern Dinka
	base("dip"): base("din"), // Northeastern Dinka
	base("diw"): base("din"), // Northwestern Dinka
	base("dks"): base("din"), // Southeastern Dinka

	// Dogri
	base("dgo"): base("doi"), // Dogri (individual language)
	base("xnr"): base("doi"), // Kangri

	// Estonian
	base("ekk"): base("et"), // Standard Estonian
	base("vro"): base("et"), // Võro

	// Persian
	base("pes"): base("fa"), // Iranian Persian
	base("prs"): base("fa"), // Dari

	// Fulah
	base("ffm"): base("ff"), // Maasina Fulfulde
	base("fub"): base("ff"), // Adamawa Fulfulde
	base("fuc"): base("ff"), // Pulaar
	base("fue"): base("ff"), // Borgu Fulfulde
	base("fuf"): base("ff"), // Pular
	base("fuh"): base("ff"), // Western Niger Fulfulde
	base("fui"): base("ff"), // Bagirmi Fulfulde
	base("fuq"): base("ff"), // Central-Eastern Niger Fulfulde
	base("fuv"): base("ff"), // Nigerian Fulfulde

	// Gbaya
	base("bdt"): base("gba"), // Bokoto
	base("gbp"): base("gba"), // Gbaya-Bossangoa
	base("gbq"): base("gba"), // Gbaya-Bozoum
	base("gmm"): base("gba"), // Gbaya-Mbodomo
	base("gso"): base("gba"), // Southwest Gbaya
	base("gya"): base("gba"), // Northwest Gbaya

	// Gondi
	base("esg"): base("gon"), // Aheri Gondi
	base("gno"): base("gon"), // Northern Gondi
	base("wsg"): base("gon"), // Adilabad Gondi

	// Grebo
	base("gbo"): base("grb"), // Northern Grebo
	base("gec"): base("grb"), // Gboloo Grebo
	base("grj"): base("grb"), // Southern Grebo
	base("grv"): base("grb"), // Central Grebo
	base("gry"): base("grb"), // Barclayville Grebo

	// Guarani
	base("gnw"): base("gn"), // Western Bolivian Guaraní
	base("gug"): base("gn"), // Paraguayan Guaraní
	base("gui"): base("gn"), // Eastern Bolivian Guaraní
	base("gun"): base("gn"), // Mbyá Guaraní
	base("nhd"): base("gn"), // Chiripá

	base("hax"): base("hai"), // Southern Haida
	base("hdn"): base("hai"), // Northern Haida

	// Serbo-Croatian
	base("bos"): base("hbs"), // Bosnian
	base("cnr"): base("hbs"), // Montenegrin
	base("hrv"): base("hbs"), // Croatian
	base("srp"): base("hbs"), // Serbian

	// Hmong
	base("cqd"): base("hmn"), // Chuanqiandian Cluster Miao
	base("hea"): base("hmn"), // Northern Qiandong Miao
	base("hma"): base("hmn"), // Southern Mashan Hmong
	base("hmc"): base("hmn"), // Central Huishui Hmong
	base("hmd"): base("hmn"), // Large Flowery Miao
	base("hme"): base("hmn"), // Eastern Huishui Hmong
	base("hmg"): base("hmn"), // Southwestern Guiyang Hmong
	base("hmh"): base("hmn"), // Southwestern Huishui Hmong
	base("hmi"): base("hmn"), // Northern Huishui Hmong
	base("hmj"): base("hmn"), // Ge
	base("hml"): base("hmn"), // Luopohe Hmong
	base("hmm"): base("hmn"), // Central Mashan Hmong
	base("hmp"): base("hmn"), // Northern Mashan Hmong
	base("hmq"): base("hmn"), // Eastern Qiandong Miao
	base("hms"): base("hmn"), // Southern Qiandong Miao
	base("hmw"): base("hmn"), // Western Mashan Hmong
	base("hmy"): base("hmn"), // Southern Guiyang Hmong
	base("hmz"): base("hmn"), // Hmong Shua
	base("hnj"): base("hmn"), // Hmong Njua
	base("hrm"): base("hmn"), // Horned Miao
	base("huj"): base("hmn"), // Northern Guiyang Hmong
	base("mmr"): base("hmn"), // Western Xiangxi Miao
	base("muq"): base("hmn"), // Eastern Xiangxi Miao
	base("mww"): base("hmn"), // Hmong Daw
	base("sfm"): base("hmn"), // Small Flowery Miao

	// Inuktitut
	base("ike"): base("iu"), // Eastern Canadian Inuktitut
	base("ikt"): base("iu"), // Inuinnaqtun

	// Inupiaq
	base("esi"): base("ik"), // North Alaskan Inupiatun
	base("esk"): base("ik"), // Northwest Alaska Inupiatun

	// Judeo-Arabic
	base("aju"): base("jrb"), // Judeo-Moroccan Arabic
	base("jye"): base("jrb"), // Judeo-Yemeni Arabic
	base("yhd"): base("jrb"), // Judeo-Iraqi Arabic
	base("yud"): base("jrb"), // Judeo-Tripolitanian Arabic

	// Kanuri
	base("kby"): base("kr"), // Manga Kanuri
	base("knc"): base("kr"), // Central Kanuri
	base("krt"): base("kr"), // Tumari Kanuri

	// Kalenjin
	base("enb"): base("kln"), // Markweeta
	base("eyo"): base("kln"), // Keiyo
	base("niq"): base("kln"), // Nandi
	base("oki"): base("kln"), // Okiek
	base("pko"): base("kln"), // Pökoot
	base("sgc"): base("kln"), // Kipsigis
	base("spy"): base("kln"), // Sabaot
	base("tec"): base("kln"), // Terik
	base("tuy"): base("kln"), // Tugen

	// Konkani
	base("gom"): base("kok"), // Goan Konkani
	base("knn"): base("kok"), // Konkani (individual language)

	// Komi
	base("koi"): base("kv"), // Komi-Permyak
	base("kpv"): base("kv"), // Komi-Zyrian

	// Kongo
	base("kng"): base("kg"), // Koongo
	base("kwy"): base("kg"), // San Salvador Kongo
	base("ldi"): base("kg"), // Laari

	// Kpelle
	base("gkp"): base("kpe"), // Guinea Kpelle
	base("xpe"): base("kpe"), // Liberia Kpelle

	// Kurdish
	base("ckb"): base("ku"), // Central Kurdish
	base("kmr"): base("ku"), // Northern Kurdish
	base("sdh"): base("ku"), // Southern Kurdish

	// Lahnda
	base("hnd"): base("lah"), // Southern Hindko
	base("hno"): base("lah"), // Northern Hindko
	base("jat"): base("lah"), // Jakati
	base("phr"): base("lah"), // Pahari-Potwari
	base("pnb"): base("lah"), // Western Panjabi
	base("skr"): base("lah"), // Saraiki
	base("xhe"): base("lah"), // Khetrani

	// Latvian
	base("ltg"): base("lv"), // Latgalian
	base("lvs"): base("lv"), // Standard Latvian

	// Luyia
	base("bxk"): base("luy"), // Bukusu
	base("ida"): base("luy"), // Idakho-Isukha-Tiriki
	base("lkb"): base("luy"), // Kabras
	base("lko"): base("luy"), // Khayo
	base("lks"): base("luy"), // Kisa
	base("lri"): base("luy"), // Marachi
	base("lrm"): base("luy"), // Marama
	base("lsm"): base("luy"), // Saamia
	base("lto"): base("luy"), // Tsotso
	base("lts"): base("luy"), // Tachoni
	base("lwg"): base("luy"), // Wanga
	base("nle"): base("luy"), // East Nyala
	base("nyd"): base("luy"), // Nyore
	base("rag"): base("luy"), // Logooli

	// Mandingo
	base("emk"): base("man"), // Eastern Maninkakan
	base("mku"): base("man"), // Konyanka Maninka
	base("mlq"): base("man"), // Western Maninkakan
	base("mnk"): base("man"), // Mandinka
	base("msc"): base("man"), // Sankaran Maninka
	base("mwk"): base("man"), // Kita Maninkakan

	// Malagasy
	base("bhr"): base("mg"), // Bara Malagasy
	base("bmm"): base("mg"), // Northern Betsimisaraka Malagasy
	base("bzc"): base("mg"), // Southern Betsimisaraka Malagasy
	base("msh"): base("mg"), // Masikoro Malagasy
	base("plt"): base("mg"), // Plateau Malagasy
	base("skg"): base("mg"), // Sakalava Malagasy
	base("tdx"): base("mg"), // Tandroy-Mahafaly Malagasy
	base("tkg"): base("mg"), // Tesaka Malagasy
	base("txy"): base("mg"), // Tanosy Malagasy
	base("xmv"): base("mg"), // Antankarana Malagasy
	base("xmw"): base("mg"), // Tsimihety Malagasy

	// Mongolian
	base("khk"): base("mn"), // Halh Mongolian
	base("mvf"): base("mn"), // Peripheral Mongolian

	// Malay
	base("bjn"): base("ms"), // Banjar
	base("btj"): base("ms"), // Bacanese Malay
	base("bve"): base("ms"), // Berau Malay
	base("bvu"): base("ms"), // Bukit Malay
	base("coa"): base("ms"), // Cocos Islands Malay
	base("dup"): base("ms"), // Duano
	base("hji"): base("ms"), // Haji
	base("ind"): base("ms"), // Indonesian
	base("jak"): base("ms"), // Jakun
	base("jax"): base("ms"), // Jambi Malay
	base("kvb"): base("ms"), // Kubu
	base("kvr"): base("ms"), // Kerinci
	base("kxd"): base("ms"), // Brunei
	base("lce"): base("ms"), // Loncong
	base("lcf"): base("ms"), // Lubu
	base("liw"): base("ms"), // Col
	base("max"): base("ms"), // North Moluccan Malay
	base("meo"): base("ms"), // Kedah Malay
	base("mfa"): base("ms"), // Pattani Malay
	base("mfb"): base("ms"), // Bangka
	base("min"): base("ms"), // Minangkabau
	base("mqg"): base("ms"), // Kota Bangun Kutai Malay
	base("msi"): base("ms"), // Sabah Malay
	base("mui"): base("ms"), // Musi
	base("orn"): base("ms"), // Orang Kanaq
	base("ors"): base("ms"), // Orang Seletar
	base("pel"): base("ms"), // Pekal
	base("pse"): base("ms"), // Central Malay
	base("tmw"): base("ms"), // Temuan
	base("urk"): base("ms"), // Urak Lawoi'
	base("vkk"): base("ms"), // Kaur
	base("vkt"): base("ms"), // Tenggarong Kutai Malay
	base("xmm"): base("ms"), // Manado Malay
	base("zlm"): base("ms"), // Malay (individual language)
	base("zmi"): base("ms"), // Negeri Sembilan Malay
	base("zsm"): base("ms"), // Standard Malay

	// Marwari
	base("dhd"): base("mwr"), // Dhundari
	base("mtr"): base("mwr"), // Mewari
	base("mve"): base("mwr"), // Marwari (Pakistan)
	base("rwr"): base("mwr"), // Marwari (India)
	base("swv"): base("mwr"), // Shekhawati
	base("wry"): base("mwr"), // Merwari

	// Nepali
	base("dty"): base("ne"), // Dotyali
	base("npi"): base("ne"), // Nepali (individual language)

	// Norwegian
	base("nno"): base("no"), // Norwegian Nynorsk
	base("nob"): base("no"), // Norwegian Bokmål

	// Ojibwa
	base("ciw"): base("oj"), // Chippewa
	base("ojb"): base("oj"), // Northwestern Ojibwa
	base("ojc"): base("oj"), // Central Ojibwa
	base("ojg"): base("oj"), // Eastern Ojibwa
	base("ojs"): base("oj"), // Severn Ojibwa
	base("ojw"): base("oj"), // Western Ojibwa
	base("otw"): base("oj"), // Ottawa

	// Oriya
	base("ory"): base("or"), // Odia
	base("spv"): base("or"), // Sambalpuri

	// Oromo
	base("gax"): base("om"), // Borana-Arsi-Guji Oromo
	base("gaz"): base("om"), // West Central Oromo
	base("hae"): base("om"), // Eastern Oromo
	base("orc"): base("om"), // Orma

	// Pashto
	base("pbt"): base("ps"), // Southern Pashto
	base("pbu"): base("ps"), // Northern Pashto
	base("pst"): base("ps"), // Central Pashto

	// Quechua
	base("qub"): base("qu"), // Huallaga Huánuco Quechua
	base("qud"): base("qu"), // Calderón Highland Quichua
	base("quf"): base("qu"), // Lambayeque Quechua
	base("qug"): base("qu"), // Chimborazo Highland Quichua
	base("quh"): base("qu"), // South Bolivian Quechua
	base("quk"): base("qu"), // Chachapoyas Quechua
	base("qul"): base("qu"), // North Bolivian Quechua
	base("qup"): base("qu"), // Southern Pastaza Quechua
	base("qur"): base("qu"), // Yanahuanca Pasco Quechua
	base("qus"): base("qu"), // Santiago del Estero Quichua
	base("quw"): base("qu"), // Tena Lowland Quichua
	base("qux"): base("qu"), // Yauyos Quechua
	base("quy"): base("qu"), // Ayacucho Quechua
	base("quz"): base("qu"), // Cusco Quechua
	base("qva"): base("qu"), // Ambo-Pasco Quechua
	base("qvc"): base("qu"), // Cajamarca Quechua
	base("qve"): base("qu"), // Eastern Apurímac Quechua
	base("qvh"): base("qu"), // Huamalíes-Dos de Mayo Huánuco Quechua
	base("qvi"): base("qu"), // Imbabura Highland Quichua
	base("qvj"): base("qu"), // Loja Highland Quichua
	base("qvl"): base("qu"), // Cajatambo North Lima Quechua
	base("qvm"): base("qu"), // Margos-Yarowilca-Lauricocha Quechua
	base("qvn"): base("qu"), // North Junín Quechua
	base("qvo"): base("qu"), // Napo Lowland Quechua
	base("qvp"): base("qu"), // Pacaraos Quechua
	base("qvs"): base("qu"), // San Martín Quechua
	base("qvw"): base("qu"), // Huaylla Wanca Quechua
	base("qvz"): base("qu"), // Northern Pastaza Quichua
	base("qwa"): base("qu"), // Corongo Ancash Quechua
	base("qwc"): base("qu"), // Classical Quechua
	base("qwh"): base("qu"), // Huaylas Ancash Quechua
	base("qws"): base("qu"), // Sihuas Ancash Quechua
	base("qxa"): base("qu"), // Chiquián Ancash Quechua
	base("qxc"): base("qu"), // Chincha Quechua
	base("qxh"): base("qu"), // Panao Huánuco Quechua
	base("qxl"): base("qu"), // Salasaca Highland Quichua
	base("qxn"): base("qu"), // Northern Conchucos Ancash Quechua
	base("qxo"): base("qu"), // Southern Conchucos Ancash Quechua
	base("qxp"): base("qu"), // Puno Quechua
	base("qxr"): base("qu"), // Cañar Highland Quichua
	base("qxt"): base("qu"), // Santa Ana de Tusi Pasco Quechua
	base("qxu"): base("qu"), // Arequipa-La Unión Quechua
	base("qxw"): base("qu"), // Jauja Wanca Quechua

	// Rajasthani
	base("bgq"): base("raj"), // Bagri
	base("gda"): base("raj"), // Gade Lohar
	base("gju"): base("raj"), // Gujari
	base("hoj"): base("raj"), // Hadothi
	base("mup"): base("raj"), // Malvi
	base("wbr"): base("raj"), // Wagdi

	// Romany
	base("rmc"): base("rom"), // Carpathian Romani
	base("rmf"): base("rom"), // Kalo Finnish Romani
	base("rml"): base("rom"), // Baltic Romani
	base("rmn"): base("rom"), // Balkan Romani
	base("rmo"): base("rom"), // Sinte Romani
	base("rmw"): base("rom"), // Welsh Romani
	base("rmy"): base("rom"), // Vlax Romani

	// Sanskrit
	// base("cls"): base("sa"), // Classical Sanskrit
	// base("vsn"): base("sa"), // Vedic Sanskrit

	// Albanian
	base("aae"): base("sq"), // Arbëreshë Albanian
	base("aat"): base("sq"), // Arvanitika Albanian
	base("aln"): base("sq"), // Gheg Albanian
	base("als"): base("sq"), // Tosk Albanian

	// Sardinian
	base("sdc"): base("sc"), // Sassarese Sardinian
	base("sdn"): base("sc"), // Gallurese Sardinian
	base("src"): base("sc"), // Logudorese Sardinian
	base("sro"): base("sc"), // Campidanese Sardinian

	// Swahili
	base("swc"): base("sw"), // Congo Swahili
	base("swh"): base("sw"), // Swahili (individual language)

	// Syriac
	base("aii"): base("syr"), // Assyrian Neo-Aramaic
	base("cld"): base("syr"), // Chaldean Neo-Aramaic

	// Tamashek
	base("taq"): base("tmh"), // Tamasheq
	base("thv"): base("tmh"), // Tahaggart Tamahaq
	base("thz"): base("tmh"), // Tayart Tamajeq
	base("ttq"): base("tmh"), // Tawallammat Tamajaq

	// Uzbek
	base("uzn"): base("uz"), // Northern Uzbek
	base("uzs"): base("uz"), // Southern Uzbek

	// Yiddish
	base("ydd"): base("yi"), // Eastern Yiddish
	base("yih"): base("yi"), // Western Yiddish

	// Zapotec
	base("zaa"): base("zap"), // Sierra de Juárez Zapotec
	base("zab"): base("zap"), // Western Tlacolula Valley Zapotec
	base("zac"): base("zap"), // Ocotlán Zapotec
	base("zad"): base("zap"), // Cajonos Zapotec
	base("zae"): base("zap"), // Yareni Zapotec
	base("zaf"): base("zap"), // Ayoquesco Zapotec
	base("zai"): base("zap"), // Isthmus Zapotec
	base("zam"): base("zap"), // Miahuatlán Zapotec
	base("zao"): base("zap"), // Ozolotepec Zapotec
	base("zaq"): base("zap"), // Aloápam Zapotec
	base("zar"): base("zap"), // Rincón Zapotec
	base("zas"): base("zap"), // Santo Domingo Albarradas Zapotec
	base("zat"): base("zap"), // Tabaa Zapotec
	base("zav"): base("zap"), // Yatzachi Zapotec
	base("zaw"): base("zap"), // Mitla Zapotec
	base("zax"): base("zap"), // Xadani Zapotec
	base("zca"): base("zap"), // Coatecas Altas Zapotec
	base("zcd"): base("zap"), // Las Delicias Zapotec
	base("zoo"): base("zap"), // Asunción Mixtepec Zapotec
	base("zpa"): base("zap"), // Lachiguiri Zapotec
	base("zpb"): base("zap"), // Yautepec Zapotec
	base("zpc"): base("zap"), // Choapan Zapotec
	base("zpd"): base("zap"), // Southeastern Ixtlán Zapotec
	base("zpe"): base("zap"), // Petapa Zapotec
	base("zpf"): base("zap"), // San Pedro Quiatoni Zapotec
	base("zpg"): base("zap"), // Guevea De Humboldt Zapotec
	base("zph"): base("zap"), // Totomachapan Zapotec
	base("zpi"): base("zap"), // Santa María Quiegolani Zapotec
	base("zpj"): base("zap"), // Quiavicuzas Zapotec
	base("zpk"): base("zap"), // Tlacolulita Zapotec
	base("zpl"): base("zap"), // Lachixío Zapotec
	base("zpm"): base("zap"), // Mixtepec Zapotec
	base("zpn"): base("zap"), // Santa Inés Yatzechi Zapotec
	base("zpo"): base("zap"), // Amatlán Zapotec
	base("zpp"): base("zap"), // El Alto Zapotec
	base("zpq"): base("zap"), // Zoogocho Zapotec
	base("zpr"): base("zap"), // Santiago Xanica Zapotec
	base("zps"): base("zap"), // Coatlán Zapotec
	base("zpt"): base("zap"), // San Vicente Coatlán Zapotec
	base("zpu"): base("zap"), // Yalálag Zapotec
	base("zpv"): base("zap"), // Chichicapan Zapotec
	base("zpw"): base("zap"), // Zaniza Zapotec
	base("zpx"): base("zap"), // San Baltazar Loxicha Zapotec
	base("zpy"): base("zap"), // Mazaltepec Zapotec
	base("zpz"): base("zap"), // Texmelucan Zapotec
	base("zsr"): base("zap"), // Southern Rincon Zapotec
	base("zte"): base("zap"), // Elotepec Zapotec
	base("ztg"): base("zap"), // Xanaguía Zapotec
	base("ztl"): base("zap"), // Lapaguía-Guivini Zapotec
	base("ztm"): base("zap"), // San Agustín Mixtepec Zapotec
	base("ztn"): base("zap"), // Santa Catarina Albarradas Zapotec
	base("ztp"): base("zap"), // Loxicha Zapotec
	base("ztq"): base("zap"), // Quioquitani-Quierí Zapotec
	base("zts"): base("zap"), // Tilquiapan Zapotec
	base("ztt"): base("zap"), // Tejalapan Zapotec
	base("ztu"): base("zap"), // Güilá Zapotec
	base("ztx"): base("zap"), // Zaachila Zapotec
	base("zty"): base("zap"), // Yatee Zapotec

	// Zhuang
	base("zch"): base("za"), // Central Hongshuihe Zhuang
	base("zeh"): base("za"), // Eastern Hongshuihe Zhuang
	base("zgb"): base("za"), // Guibei Zhuang
	base("zgm"): base("za"), // Minz Zhuang
	base("zgn"): base("za"), // Guibian Zhuang
	base("zhd"): base("za"), // Dai Zhuang
	base("zhn"): base("za"), // Nong Zhuang
	base("zlj"): base("za"), // Liujiang Zhuang
	base("zln"): base("za"), // Lianshan Zhuang
	base("zlq"): base("za"), // Liuqian Zhuang
	base("zqe"): base("za"), // Qiubei Zhuang
	base("zyb"): base("za"), // Yongbei Zhuang
	base("zyg"): base("za"), // Yang Zhuang
	base("zyj"): base("za"), // Youjiang Zhuang
	base("zyn"): base("za"), // Yongnan Zhuang
	base("zzj"): base("za"), // Zuojiang Zhuang

	// Chinese
	base("cdo"): base("zh"), // Min Dong Chinese
	base("cjy"): base("zh"), // Jinyu Chinese
	base("cmn"): base("zh"), // Mandarin Chinese
	base("cnp"): base("zh"), // Northern Ping Chinese
	base("cpx"): base("zh"), // Pu-Xian Chinese
	base("csp"): base("zh"), // Southern Ping Chinese
	base("czh"): base("zh"), // Huizhou Chinese
	base("czo"): base("zh"), // Min Zhong Chinese
	base("gan"): base("zh"), // Gan Chinese
	base("hak"): base("zh"), // Hakka Chinese
	base("hsn"): base("zh"), // Xiang Chinese
	base("lzh"): base("zh"), // Literary Chinese
	base("mnp"): base("zh"), // Min Bei Chinese
	base("nan"): base("zh"), // Min Nan Chinese
	base("wuu"): base("zh"), // Wu Chinese
	base("yue"): base("zh"), // Yue Chinese

	// Zaza
	base("diq"): base("zza"), // Dimli (individual language)
	base("kiu"): base("zza"), // Kirmanjki (individual language)
}
