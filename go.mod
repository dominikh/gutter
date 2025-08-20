// SPDX-FileCopyrightText: none
// SPDX-License-Identifier: CC0-1.0

module honnef.co/go/gutter

go 1.24

require (
	github.com/go-json-experiment/json v0.0.0-20240815175050-ebd3a8989ca1
	github.com/go-text/typesetting v0.2.1
	github.com/google/go-cmp v0.6.0
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8
	golang.org/x/sys v0.31.0
	golang.org/x/text v0.21.0
	golang.org/x/tools v0.31.0
	honnef.co/go/color v0.0.0-20250521155844-50a57575c7d3
	honnef.co/go/curve v0.0.0-20250325031802-e021cd9ef495
	honnef.co/go/jello v0.0.0-20250521161253-6b746b01ce5a
	honnef.co/go/libwayland v0.0.0-20250604131836-2b956ec1bdb1
	honnef.co/go/safeish v0.0.0-20241114181457-67c0a2c357ad
	honnef.co/go/stuff v0.0.0-20250719175023-de3141074b7c
)

require (
	github.com/mmcloughlin/avo v0.6.0 // indirect
	golang.org/x/image v0.3.0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
)

tool (
	github.com/mmcloughlin/avo
	golang.org/x/tools/cmd/stringer
)
