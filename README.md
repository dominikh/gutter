<!-- SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors -->
<!-- SPDX-License-Identifier: MIT -->

# Gutter – A declarative UI framework for Go

Gutter is a declarative and reactive UI framework that is heavily inspired by
Flutter, but aims to present an idiomatic Go API. That is, it is not a clone.

## Current status

The project is currently in the prototyping stage.

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
