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
