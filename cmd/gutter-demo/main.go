// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"image/color"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"

	"gioui.org/app"
	giopointer "gioui.org/io/pointer"
	"gioui.org/op"
)

func main() {
	log.SetFlags(log.Lmicroseconds)
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

func (cs *colorChangingBoxState) Build(ctx widget.BuildContext) widget.Widget {
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

var win *app.Window

func run2(w *app.Window) error {
	win = w

	root := &widget.Builder{
		Builder: func(ctx widget.BuildContext, _ widget.Widget) widget.Widget {
			_ = widget.DependOnWidgetOfExactType[*widget.MediaQuery](ctx).Data.Size.X
			now := rand.Int()
			return &widget.ColoredBox{
				Color: color.NRGBA{uint8(now), 0, 0, 255},
				// Color: color.NRGBA{uint8(rand.Int()), 0, 0, 255},
			}
		},
	}

	b := widget.RunApp(w, root)

	var ops op.Ops
	prev := time.Now()
	prevRender := time.Now()
	for {
		e := w.NextEvent()
		now := time.Now()
		d := now.Sub(prev)
		log.Printf("%T; %s since last event", e, d)
		prev = now
		switch e := e.(type) {
		default:
			// fmt.Printf("%T %v\n", e, e)
		case giopointer.Event:
			b.RenderBinding.HandlePointerEvent(e)
		case app.DestroyEvent:
			return e.Err
		case widget.CallbackEvent:
			e()
		case app.FrameEvent:
			// log.Println("--frame--", e.Size)
			d := now.Sub(prevRender)
			prevRender = now
			log.Println(d, "since last frame")
			t := time.Now()
			b.DrawFrame(e, &ops)
			log.Println("drew frame in", time.Since(t))
		}
	}
}
