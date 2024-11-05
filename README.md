<!-- SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors -->
<!-- SPDX-License-Identifier: MIT -->

# Gutter – A declarative UI framework for Go

Gutter is a declarative and reactive UI framework that is heavily inspired by
Flutter, but aims to present an idiomatic Go API. That is, it is not a clone.

## Current status

The project is currently in the prototyping stage.

## Features

- Support for the [Unicode Line Breaking Algorithm (UAX #14)](https://unicode.org/reports/tr14/)

## Design choices

### Desktop-only

We believe that desktop and mobile UIs have vastly different needs and that
trying to serve both in one framework leads to a subpar experience for at least
one of them. As such, this framework exclusively supports desktop operating
systems; mobile OS support is out of scope.

### GPU-accelerated rendering

Gutter uses [Jello](https://github.com/dominikh/jello) as its renderer. Jello is
a Go port of [Vello](https://github.com/linebender/vello) and uses compute
shaders to implement a fast and efficient 2D vector renderer.

We want users to embrace the power of having fast, scalable 2D rendering.
Because even the gap between the slowest and fastest GPU is smaller than the gap
between CPU and GPU rendering, we do not offer a CPU fallback renderer.

Gutter requires a GPU that supports at least Vulkan 1.2, D3D12, or Metal. This
should be covered by GPUs of the last 10 years.

### Inspired by, but not a clone of, Flutter

Gutter is heavily inspired by Flutter, which in the author's opinion is a solid implementation of the
declarative UI pattern. To arrive at a usable prototype faster, large parts of Gutter's initial design were
copied directly from Flutter, with adjustments to make its API more suited to Go's type system. However, it is
not the goal to provide a faithful Flutter clone. In fact, as the project progresses, it will likely move
further away from Flutter's API, as Gutter is made more Go-like, and as abstractions that are deemed
unnecessary are removed.

### Modern font handling

- TODO
- Not restricted to the WWS or R/B/I/BI models, instead embracing variable fonts and unrestricted font models. we try
  our best to map old-fashioned fonts to our flexible API.
- not relying on fontconfig
- not supporting old encodings like the Macintosh code pages, Big5, etc. Unicode or go home.

#### Font fallback

In DTP and related typesetting tasks, users will generally explicitly select
desirable fonts that cover all of the text they need to display, and embed these
fonts in the document.

GUIs, like websites, do not have this affordance, if they have to display
externally controlled text (e.g. user-generated content). It is not feasible to
have an opinion on fonts to use for every single script and language, and even
if it were, it is not feasible to bundle all of the fonts due to size
constraints. At best, we can specify a limited set of fonts for the scripts we
care about the most. For the rest, we have to rely on automatic fallback to
whatever fonts the user has available. For example, a Persian user will have
adequate fonts covering the Arabic script installed on their system, even if we
didn't choose a specific font for them.

Different fonts can, however, have vastly different appearances due to style and
metrics. Ideally, when selecting fallback fonts, we'd select fonts that match
our overall style.

It is also important to differentiate between scripts and languages, as multiple
languages use the same script, but with different visual appearances. For
example, Russian and Ukrainian Cyrillic have differently looking glyphs, but
they use the same Unicode code points. See also [Han
unification](https://en.wikipedia.org/wiki/Han_unification). Often we won't know
the language of some text, but when we do, it'd be highly desirable to select
the right font, and we should probably fall back to the user's locale to break
ties in any case.

Ideally, the operating system would provide us with the necessary APIs for
selecting fonts based on style, metrics, script, language, and Unicode coverage
(as fonts may support subsets of a script only).

- TODO font metadata (it's hella inconsistent)
- TODO fontforge configuration (it's hella lacking)
- TODO fontforge API (doesn't use BCP 47 tags, making selecting by script harder)

We _could_ depend on Noto fonts and forego font fallback altogether, requiring
users to install the appropriate Noto fonts if they want to be able to see some
script. But we can't make that a hard requirement, as all fonts combined are
hundreds of MB large. Additionally, a lot of users have strong opinions on fonts
and want to use their preferred font, not the default. Furthermore, programs
might use custom fonts with custom symbols or emoji.

- TODO mixed script text, and what do we do about Latin embedded in CKJ text?
  use our default Latin font, or use the Latins from the CKJ font?

##### Font metadata

The OS/2 table has 127 bits for describing blocks of Unicode that are
"functional". It is not defined what that means, whether all code points in a
block have to be supported or if not then how many, etc. Different font creation
tools interpret it differently. Also, as it operates on Unicode blocks, it
cannot make any statements on _language_ support (and blocks aren't a perfect
stand-in for scripts, either). Also, as of Unicode 16, there are 338 blocks,
which is more than 127. Thus, when we actually care about Unicode coverage, we
have to scan cmap tables and can't rely on the bits.

The OS/2 table has the `sFamilyClass` field (the IBM Font Family
Classification), which classifies a font based on its style and substyle. This
field is very Western-specific, with 6 styles dedicated to different kinds of
serifs (each with their own substyles!), 1 for sans serif, 1 for ornamentals, 1
for scripts (i.e. handwriting) and 1 for symbolic fonts. The field cannot
accurately classify fonts for Asian, Arabic, Indian, or other scripts. A lot of
fonts populate this field, but a lot of fonts don't. Even within one family of
fonts, use of the field can be spotty: all variants of IBM Plex Sans specify it,
with the exception of IBM Plex Sans Condensed. What's special about the
condensed font? I don't know.

The OS/2 table has the `panose` field which is the PANOSE classification number.
This is similar so `sFamilyClass` but provides more fine-grained information
about a font's visual appearance. As specified, it only covers Latin fonts. It
is not clear whether use of PANOSE requires a license from Hewlett-Packard.
Surprisingly, the majority of fonts we checked provide this information. It is
not clear, however, whether they provide _accurate_ information. Discussions on
the internet (e.g.
https://typedrawers.com/discussion/1299/automating-panose-data) suggest that
nobody actually cares about PANOSE and a lot of values are cargo culted or
guesstimated. Apparently there is
https://github.com/googlefonts/panose-devanagari from 2014 for extending PANOSE
to cover Devanagari. Not even Google's Noto fonts use it.
https://lists.freedesktop.org/archives/fontconfig/2019-June/006528.html says:
"On my system out of 7099 fonts only 975 contain Panose information."

Neither sFamilyClass nor panose seem to be covered by any tables for variable
fonts.

OpenType 1.8 (2016) added the `meta` table and the `dlng` and `slng` tags. These
specify the languages or scripts a font has been designed for, and the languages
or scripts that a font supports, using BCP 47 tags. For example, a Japanese font
will have been designed for Japanese, but probably also have basic support for
Latin.

The meta table isn't commonly used. Out of all Noto fonts, only one has it. Web
core fonts (such as Arial) don't have it because they predate it and don't get
updated anymore. The same is true for a lot of established open source fonts, as
they're older than OpenType 1.8 and are considered "done". IBM Plex fonts do
have meta tables.

- TODO other contexts where we want rich metadata: applications that let users
  select fonts. we want a font selector that can filter by styles and language
  support and whatnot.

### Pixel-aligned UI elements

TODO

### Native, but not _native_

Go compiles to machine code and Gutter talks directly to the operating system.
This makes Gutter--compared to browser-based UI frameworks like
Electron--native.

We do not, however, use the "native" GUI toolkits of the operating systems.
Using native toolkits is portrayed as being desirable to achieve a uniform look
& feel. However, out of Linux, Windows, and macOS, only macOS truly has a native
toolkit. Linux at a minimum has GTK and Qt (and every major version of these
feels decidely different from previous versions and is backwards incompatible),
but also less-used options such as Motif. Windows has the Win32 API, WinForms,
WPF, UWP, WinUI 3, and DirectUI. A modern Windows installation will have
applications using all of these frameworks. It is therefore inherently
impossible to achieve a uniform experience on Linux or Windows.

We could have chosen one "native" toolkit per platform and tried to hide their
differences (à la wxWidgets), or used GTK or Qt to feel "native" on at
least one platform. However, Gutter is ultimately an experimental project that
tries to improve upon current GUIs by innovating, not by being limited to the
lowest commen denominator.

## Known shortcomings that may be fixed by contributors

- No support for correct line breaking of Thai

## Known shortcomings that are intentional

- No support for touch gestures
- No support for mobile devices
- No support for text directions other than left-to-right and right-to-left
