package widget

import (
	"gioui.org/io/pointer"
	"honnef.co/go/gutter/gesture"
	"honnef.co/go/gutter/render"
)

var _ SingleChildWidget = (*GestureDetector)(nil)

type GestureDetector struct {
	OnTap       func(ev pointer.Event)
	OnDoubleTap func(ev pointer.Event)
	Child       Widget
}

// CreateElement implements SingleChildWidget.
func (g *GestureDetector) CreateElement() Element {
	return NewInteriorElement(g)
}

func (g *GestureDetector) CreateState() State {
	return &gestureDetectorState{}
}

// GetChild implements SingleChildWidget.
func (g *GestureDetector) GetChild() Widget {
	return g.Child
}

// Key implements SingleChildWidget.
func (g *GestureDetector) Key() any {
	// XXX implement
	return nil
}

type gestureDetectorState struct {
	StateHandle

	recognizers []gesture.Recognizer
}

// Transition implements State.
func (g *gestureDetectorState) Transition(t StateTransition) {
	if t.Kind == StateInitializing {
		if onTap := g.Widget.(*GestureDetector).OnTap; onTap != nil {
			g.recognizers = append(g.recognizers, &gesture.TapRecognizer{
				Manager: gesture.ARENA_MANAGER,
				OnTap:   onTap,
			})
		}
		if onDtap := g.Widget.(*GestureDetector).OnDoubleTap; onDtap != nil {
			g.recognizers = append(g.recognizers, &gesture.DoubleTapRecognizer{
				Manager:     gesture.ARENA_MANAGER,
				OnDoubleTap: onDtap,
			})
		}
	}
}

func (g *gestureDetectorState) Build() Widget {
	return &PointerRegion{
		OnAll: func(hit render.HitTestEntry, ev pointer.Event) {
			for _, rec := range g.recognizers {
				rec.HandlePointerEvent(ev)
			}
		},
	}
}
