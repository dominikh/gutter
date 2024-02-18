// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package debug

import rdebug "runtime/debug"

// XXX move this behind a build tag
const debug = true

func Assert(b bool) {
	if !b {
		panic("failed assertion")
	}
}

var PrintStack = rdebug.PrintStack
