// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package animation

import "fmt"

func ExampleTween_linear() {
	tween := Tween[int]{
		Start:   10,
		End:     20,
		Compute: Lerp[int],
	}
	fmt.Println(tween.Evaluate(0.5))
	// Output:
	// 15
}
