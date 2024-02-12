package pointer

import (
	"fmt"
	"time"

	"honnef.co/go/gutter/f32"

	"gioui.org/io/key"
	giopointer "gioui.org/io/pointer"
)

type Event struct {
	Kind      Kind
	Priority  Priority
	Time      time.Duration
	Buttons   Buttons
	Position  f32.Point
	Scroll    f32.Point
	Modifiers key.Modifiers
}

func FromRaw(ev giopointer.Event) Event {
	var kind Kind
	switch ev.Kind {
	case giopointer.Cancel:
		kind = Cancel
	case giopointer.Press:
		kind = Press
	case giopointer.Release:
		kind = Release
	case giopointer.Move:
		kind = Move
	case giopointer.Scroll:
		kind = Scroll
	default:
		panic(fmt.Sprintf("unhandled kind %#x", ev.Kind))
	}

	return Event{
		Kind:      kind,
		Time:      ev.Time,
		Buttons:   Buttons(ev.Buttons),
		Position:  f32.Point(ev.Position),
		Scroll:    f32.Point(ev.Scroll),
		Modifiers: ev.Modifiers,
	}
}

type Kind uint8

const (
	Cancel Kind = 1 << iota
	Press
	Release
	Move
	Enter
	Leave
	Scroll
)

type Priority uint8

const (
	// Shared priority is for handlers that are part of a matching set larger than 1.
	Shared Priority = 1 << iota
	// Foremost priority is like Shared, but the handler is the foremost of the matching set.
	Foremost
	// Exclusive is used for matching sets of size 1.
	Exclusive
)

type Buttons uint32

const (
	// ButtonPrimary is the primary button, usually the left button for a
	// right-handed user.
	ButtonPrimary Buttons = 1 << iota
	// ButtonSecondary is the secondary button, usually the right button for a
	// right-handed user.
	ButtonSecondary
	// ButtonTertiary is the tertiary button, usually the middle button.
	ButtonTertiary
)
