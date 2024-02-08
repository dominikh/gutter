package main

// Right now we have a weird mix of designs. Gio wants to push us events and get frames in return. The Flutter
// design uses callbacks and sort of expects frames to be pushed. We should combine the two. Also, the
// render.View abstraction is a bit silly right now, considering we pass op.Ops to WidgetsBinding.DrawFrame.
// render.View is supposed to wrap a FlutterView, which provides functionality for pushing frames into. We
// should probably store an op.Ops in the view, and overall don't expose Gio's event loop to the user. In the
// end, all of the Gio events should be "reactive".

import (
	"image/color"
	"log"
	"os"
	"time"

	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
)

func main() {
	go func() {
		w := app.NewWindow()
		err := run2(w)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

var _ widget.Widget = (*Bird)(nil)

type Bird struct {
	color color.NRGBA
}

func (w *Bird) CreateElement() widget.Element {
	return widget.NewInteriorElement(w)
}

func (w *Bird) CreateState() widget.State {
	return &BirdState{}
}

type BirdState struct {
	widget.StateHandle

	c color.NRGBA
}

func (s *BirdState) Transition(t widget.StateTransition) {
	switch t.Kind {
	case widget.StateInitializing:
		s.c = color.NRGBA{0, 255, 0, 255}
		go ticker(1*time.Millisecond, func() {
			s.c.R += 1
			widget.MarkNeedsBuild(s.Element)
		})

	}
	// XXX tear down the timer when we're done
}

func (s *BirdState) Build() widget.Widget {
	return &widget.Padding{
		Padding: render.Inset{20, 20, 20, 20},
		Child: &widget.ColoredBox{
			Color: color.NRGBA{255, 0, 0, 255},
			Child: &widget.Padding{
				Padding: render.Inset{100, 100, 100, 100},
				Child: &widget.ColoredBox{
					Color: s.c,
				},
			},
		},
	}
}

// Key implements widget.StatefulWidget.
func (*Bird) Key() any {
	return nil
}

var notify = make(chan func(), 1)
var win *app.Window

func ticker(interval time.Duration, fn func()) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		notify <- fn
		win.Invalidate()
	}
}

func run2(w *app.Window) error {
	win = w

	var root widget.Widget = &Bird{
		color: color.NRGBA{0, 255, 0, 255},
	}

	// This is basically runApp
	var bo widget.BuildOwner
	po := render.NewPipelineOwner()
	wview := widget.NewView(root, po)
	rootElem := wview.Attach(&bo, nil)

	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			cs := render.ViewConfiguration{
				Min: f32.FPt(e.Size),
				Max: f32.FPt(e.Size),
			}
			rootElem.SetConfiguration(cs)

		loop:
			for {
				select {
				case fn := <-notify:
					fn()
				default:
					break loop
				}
			}

			// XXX we need to get the current constraints into the view
			bo.BuildScope(rootElem, nil)
			po.FlushLayout()
			po.FlushCompositingBits()
			po.FlushPaint(&ops)
			bo.FinalizeTree()
			e.Frame(&ops)
		}
	}
}
