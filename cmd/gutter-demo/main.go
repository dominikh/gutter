// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
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

	ch := make(chan color.NRGBA)
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		for range t.C {
			ch <- color.NRGBA{
				uint8(rand.Int()),
				uint8(rand.Int()),
				uint8(rand.Int()),
				255,
			}
		}
	}()
	root := &widget.ChannelBuilder[color.NRGBA]{
		Channel: ch,
		Builder: func(ctx widget.BuildContext, _ widget.Widget, v color.NRGBA) widget.Widget {
			fmt.Println(v)
			return &widget.ColoredBox{
				Color: v,
			}
		},
	}

	b := widget.RunApp(w, root)

	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		default:
			// fmt.Printf("%T %v\n", e, e)
		case giopointer.Event:
			b.RenderBinding.HandlePointerEvent(e)
		case app.DestroyEvent:
			return e.Err
		case widget.CallbackEvent:
			e()
		case app.FrameEvent:
			fmt.Println("--frame--")
			b.DrawFrame(e, &ops)
		}
	}
}
