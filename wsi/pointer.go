// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

// Our pointer event APIs have been inspired by [1], [2], as well as Gio.
//
// [1]: https://www.w3.org/TR/pointerevents3/
// [2]: https://github.com/rust-windowing/winit/issues/3833

package wsi

type PointerKind int

const (
	PointerKindUnknown PointerKind = iota
	PointerKindMouse
	PointerKindPen
	PointerKindTouch
)

type PointerButton int

const (
	// Left mouse button, touch/pen contact, etc.
	PointerButtonPrimary PointerButton = iota + 1
	// Middle mouse button
	PointerButtonAuxiliary
	// Right mouse button, pen barrel button, etc
	PointerButtonSecondary
	// The back button on a mouse
	PointerButtonBack
	// The forward button on a mouse
	PointerButtonForward
	// The eraser on a pen
	PointerButtonPenEraser
)

type PointerEvent struct {
	PointerID                    int
	PointerKind                  PointerKind
	Width, Height                float64
	Pressure, TangentialPressure float64
	TiltX, TiltY                 float64
	Twist                        float64
	AltitudeAngle, AzimuthAngle  float64
	IsPrimary                    bool

	// XXX do we need both tilt and altitude? they describe the same position,
	// just differently. The W3C spec even provides formulas for converting
	// between them.

	// The position of the pointer event, in logical window coordinates.
	X, Y float64
	// The keyboard modifiers that were active when this event occurred.
	Modifiers uint64
	// The button that was pressed or released to cause this event, -1 if the
	// event wasn't caused by a button.
	Button PointerButton
	// The buttons that were pressed when this event occurred, as a bitmask. For
	// PointerEnter events, this field might not be set even if buttons were
	// pressed when the pointer entered the window.
	Buttons uint64
}

type PointerDown PointerEvent

// XXX
//
// PointerUp events might occur without corresponding PointerDown events if
// buttons were already pressed when the pointer entered the window.
type PointerUp PointerEvent

type PointerEnter PointerEvent
type PointerLeave PointerEvent
type PointerMove PointerEvent
type PointerCancelled PointerEvent

type SwipeGestureBeginEvent struct{}
type SwipeGestureUpdateEvent struct{}
type SwipeGestureEndEvent struct{}

type PinchGestureBeginEvent struct{}
type PinchGestureUpdateEvent struct{}
type PinchGestureEndEvent struct{}

type HoldGestureBeginEvent struct{}
type HoldGestureEndEvent struct{}
