package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"

	"gioui.org/app"
	giopointer "gioui.org/io/pointer"
	"gioui.org/op"
)

func main() {
	// runtime.MemProfileRate = 1
	go func() {
		cpuf, _ := os.Create("cpu.pprof")
		pprof.StartCPUProfile(cpuf)
		w := app.NewWindow(app.CustomInputHandling(true))
		err := run2(w)
		if err != nil {
			log.Fatal(err)
		}
		f, _ := os.Create("mem.pprof")
		pprof.StopCPUProfile()
		runtime.GC()
		pprof.WriteHeapProfile(f)
		os.Exit(0)
	}()
	app.Main()
}

var _ widget.Widget = (*PointlessWrapper)(nil)

type PointlessWrapper struct {
	Child widget.Widget
}

// CreateElement implements widget.Widget.
func (p *PointlessWrapper) CreateElement() widget.Element {
	return widget.NewProxyElement(p)
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
	reparent bool
}

func (s *BirdState) Transition(t widget.StateTransition[*Bird]) {}

func (s *BirdState) Build() widget.Widget {
	left := &widget.KeyedSubtree{
		Key: widget.GlobalKey{"left"},
		Child: &widget.Flex{
			Direction: render.Vertical,
			Children: []widget.Widget{
				&widget.Flexible{
					Flex:  3,
					Child: &ColorChangingBox{color.NRGBA{255, 0, 0, 255}},
				},
				&widget.Flexible{
					Flex:  1,
					Child: &ColorChangingBox{color.NRGBA{0, 255, 0, 255}},
				},
				&widget.Flexible{
					Flex:  1,
					Child: &ColorChangingBox{color.NRGBA{0, 0, 255, 255}},
				},
			},
		},
	}
	right := &widget.KeyedSubtree{
		Key: widget.GlobalKey{"right"},
		Child: &widget.Flex{
			Direction: render.Vertical,
			Children: []widget.Widget{
				&widget.Flexible{
					Flex:  3,
					Child: &ColorChangingBox{color.NRGBA{255, 0, 0, 255}},
				},
				&widget.Flexible{
					Flex:  1,
					Child: &ColorChangingBox{color.NRGBA{0, 255, 0, 255}},
				},
				&widget.Flexible{
					Flex:  1,
					Child: &ColorChangingBox{color.NRGBA{0, 0, 255, 255}},
				},
			},
		},
	}
	var child widget.Widget
	if s.reparent {
		c := &left.Child.(*widget.Flex).Children
		*c = append(*c, &widget.Flexible{Flex: 2, Child: right})
		child = &widget.Flex{
			Direction: render.Horizontal,
			Children:  []widget.Widget{left},
		}
	} else {
		child = &widget.Flex{
			Direction: render.Horizontal,
			Children:  []widget.Widget{left, right},
		}
	}
	return &widget.PointerRegion{
		OnPress: func(hit render.HitTestEntry, ev pointer.Event) {
			if ev.Priority == pointer.Exclusive {
				s.reparent = !s.reparent
				widget.MarkNeedsBuild(s.Element)
			}
		},
		Child: child,
	}
}

type ColorChangingBox struct {
	Color color.NRGBA
}

func (c *ColorChangingBox) CreateElement() widget.Element {
	return widget.NewInteriorElement(c)
}

func (c *ColorChangingBox) CreateState() widget.State[*ColorChangingBox] {
	return &colorChangingBoxState{c: c.Color}
}

type colorChangingBoxState struct {
	widget.StateHandle[*ColorChangingBox]
	c color.NRGBA
}

func (cs *colorChangingBoxState) Transition(t widget.StateTransition[*ColorChangingBox]) {
}

func (cs *colorChangingBoxState) Build() widget.Widget {
	return &widget.PointerRegion{
		OnPress: func(hit render.HitTestEntry, ev pointer.Event) {
			cs.c.R += 50
			widget.MarkNeedsBuild(cs.Element)
		},
		Child: &widget.SizedBox{
			Width: 100,
			Child: &widget.ColoredBox{
				Color: cs.c,
			},
		},
	}
}

// func (s *BirdState) Build() widget.Widget {
// 	tree := &widget.KeyedSubtree{
// 		Key: "birdie",
// 		Child: &widget.AnimatedPadding{
// 			Duration: 1000 * time.Millisecond,
// 			Curve:    animation.EaseOutBounce,
// 			Padding:  render.Inset{s.padding, s.padding, s.padding, s.padding},
// 			Child: &PointlessWrapper{
// 				Child: &widget.PointerRegion{
// 					OnMove: func(hit render.HitTestEntry, ev pointer.Event) {
// 						// fmt.Println("outer:", ev)
// 					},
// 					Child: &widget.ColoredBox{
// 						Color: color.NRGBA{255, 0, 0, 255},
// 						Child: &widget.Padding{
// 							Padding: render.Inset{200, 200, 200, 200},
// 							Child: &widget.AnimatedOpacity{
// 								Opacity:  s.opacity,
// 								Duration: time.Second,
// 								Child: &PointlessWrapper{
// 									Child: &widget.PointerRegion{
// 										OnPress: func(hit render.HitTestEntry, ev pointer.Event) {
// 											if s.padding == 200 {
// 												s.padding = 0
// 											} else {
// 												s.padding = 200
// 											}
// 											if s.opacity == 1 {
// 												s.opacity = 0
// 											} else {
// 												s.opacity = 1
// 											}
// 											widget.MarkNeedsBuild(s.Element)
// 										},
// 										OnMove: func(hit render.HitTestEntry, ev pointer.Event) {
// 											// fmt.Println("inner:", ev)
// 											// s.c.G = uint8(hit.Offset.Y)
// 											// widget.MarkNeedsBuild(s.Element)
// 										},
// 										Child: &widget.ColoredBox{
// 											Color: s.c,
// 										},
// 									},
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}

// 	return tree
// }

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
	bo := widget.NewBuildOwner()
	po := render.NewPipelineOwner()
	bo.PipelineOwner = po
	wview := widget.NewView(root, po)
	rootElem := wview.Attach(bo, nil)

	po.OnNeedVisualUpdate = win.Invalidate
	bo.OnBuildScheduled = win.Invalidate

	var ht render.HitTestResult
	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		default:
			// fmt.Printf("%T %v\n", e, e)
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
			fmt.Println("--frame--")
			// fmt.Println(widget.FormatElementTree(rootElem))
		}
	}
}
