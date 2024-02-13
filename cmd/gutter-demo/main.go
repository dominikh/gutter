package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"

	"gioui.org/app"
	giopointer "gioui.org/io/pointer"
	"gioui.org/op"
)

func main() {
	runtime.MemProfileRate = 1
	go func() {
		w := app.NewWindow(app.CustomInputHandling(true))
		err := run2(w)
		if err != nil {
			log.Fatal(err)
		}
		f, _ := os.Create("mem.pprof")
		runtime.GC()
		pprof.WriteHeapProfile(f)
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

func (w *Bird) CreateState() widget.State[*Bird] {
	return &BirdState{}
}

type BirdState struct {
	widget.StateHandle[*Bird]

	padding float32
	c       color.NRGBA
}

func (s *BirdState) Transition(t widget.StateTransition[*Bird]) {
	switch t.Kind {
	case widget.StateInitializing:
		s.c = color.NRGBA{0, 255, 0, 255}
		s.padding = 200
		// go ticker(1*time.Millisecond, func() {
		// 	s.c.R += 1
		// })

	}
	// XXX tear down the timer when we're done
}

func (s *BirdState) Build() widget.Widget {
	return &widget.AnimatedPadding{
		Duration: 1000 * time.Millisecond,
		Curve:    animation.EaseOutBounce,
		Padding:  render.Inset{s.padding, s.padding, s.padding, s.padding},
		Child: &widget.PointerRegion{
			OnMove: func(hit render.HitTestEntry, ev pointer.Event) {
				// fmt.Println("outer:", ev)
			},
			Child: &widget.ColoredBox{
				Color: color.NRGBA{255, 0, 0, 255},
				Child: &widget.Padding{
					Padding: render.Inset{200, 200, 200, 200},
					Child: &widget.PointerRegion{
						OnPress: func(hit render.HitTestEntry, ev pointer.Event) {
							if s.padding == 200 {
								s.padding = 0
							} else {
								s.padding = 200
							}
							widget.MarkNeedsBuild(s.Element)
						},
						OnMove: func(hit render.HitTestEntry, ev pointer.Event) {
							// fmt.Println("inner:", ev)
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
	bo.PipelineOwner = po
	wview := widget.NewView(root, po)
	rootElem := wview.Attach(&bo, nil)

	po.OnNeedVisualUpdate = win.Invalidate
	bo.OnBuildScheduled = win.Invalidate

	var ht render.HitTestResult
	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		default:
			fmt.Printf("%T %v\n", e, e)
		case giopointer.Event:
			ht.Reset()
			render.HitTest(&ht, rootElem.RenderHandle().RenderObject, e.Position)
			n := 0
			for _, hit := range ht.Hits {
				if _, ok := hit.Object.(render.PointerEventHandler); ok {
					n++
					if n >= 2 {
						break
					}
				}
			}
			var kind pointer.Priority
			if n < 2 {
				kind = pointer.Exclusive
			} else {
				kind = pointer.Shared
			}
			first := true
			for _, hit := range ht.Hits {
				if obj, ok := hit.Object.(render.PointerEventHandler); ok {
					prio := kind
					if first && prio == pointer.Shared {
						prio = pointer.Foremost
					}
					first = false
					ev := pointer.FromRaw(e)
					ev.Priority = prio
					obj.HandlePointerEvent(hit, ev)
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

			po.RunFrameCallbacks(e.Now)

			bo.BuildScope(rootElem, nil)
			po.FlushLayout()
			po.FlushCompositingBits()
			po.FlushPaint(&ops)
			bo.FinalizeTree()
			e.Frame(&ops)
		}
	}
}
