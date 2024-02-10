package main

// Right now we have a weird mix of designs. Gio wants to push us events and get frames in return. The Flutter
// design uses callbacks and sort of expects frames to be pushed. We should combine the two. Also, the
// render.View abstraction is a bit silly right now, considering we pass op.Ops to WidgetsBinding.DrawFrame.
// render.View is supposed to wrap a FlutterView, which provides functionality for pushing frames into. We
// should probably store an op.Ops in the view, and overall don't expose Gio's event loop to the user. In the
// end, all of the Gio events should be "reactive".

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"time"

	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"

	"gioui.org/app"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

func main() {
	go func() {
		w := app.NewWindow(app.CustomInputHandling(true))
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
		// go ticker(1*time.Millisecond, func() {
		// 	s.c.R += 1
		// })

	}
	// XXX tear down the timer when we're done
}

func (s *BirdState) Build() widget.Widget {
	return &widget.Padding{
		Padding: render.Inset{20, 20, 20, 20},
		Child: &widget.Padding{
			Padding: render.Inset{50, 50, 50, 50},
			Child: &widget.ColoredBox{
				Color: color.NRGBA{255, 0, 0, 255},
				Child: &widget.Padding{
					Padding: render.Inset{200, 200, 200, 200},
					Child: &widget.PointerRegion{
						OnMove: func(hit render.HitTestEntry, ev pointer.Event) {
							s.c.G = uint8(hit.Offset.Y)
							widget.MarkNeedsBuild(s.Element)
						},
						Child: &widget.ColoredBox{
							Color: s.c,
						},
					},
				},
			},
		},
	}
}

// Key implements widget.StatefulWidget.
func (*Bird) Key() any {
	return nil
}

var win *app.Window

func ticker(interval time.Duration, fn func()) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		win.EmitEvent(CallbackEvent{fn})
	}
}

type CallbackEvent struct {
	cb func()
}

func (CallbackEvent) ImplementsEvent() {}

func run2(w *app.Window) error {
	win = w

	var root widget.Widget = &Bird{}

	// This is basically runApp
	var bo widget.BuildOwner
	po := render.NewPipelineOwner()
	wview := widget.NewView(root, po)
	rootElem := wview.Attach(&bo, nil)

	bo.OnBuildScheduled = func() {
		win.Invalidate()
	}

	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		default:
			fmt.Printf("%T %v\n", e, e)
		case pointer.Event:
			var ht render.HitTestResult
			render.HitTest(&ht, rootElem.RenderHandle().RenderObject, e.Position)
			for _, hit := range ht.Hits {
				if obj, ok := hit.Object.(render.PointerEventHandler); ok {
					obj.HandlePointerEvent(hit, e)
				}
			}

			// fmt.Println(ht)
		case app.DestroyEvent:
			return e.Err
		case CallbackEvent:
			e.cb()
		case app.FrameEvent:
			ops.Reset()
			cs := render.ViewConfiguration{
				Min: f32.FPt(e.Size),
				Max: f32.FPt(e.Size),
			}
			rootElem.SetConfiguration(cs)

			bo.BuildScope(rootElem, nil)
			po.FlushLayout()
			po.FlushCompositingBits()
			po.FlushPaint(&ops)
			bo.FinalizeTree()
			e.Frame(&ops)
		}
	}
}
