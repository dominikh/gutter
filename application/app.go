// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package application

import (
	"context"
	"fmt"
	"log"
	"time"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/sparse"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/gutter/widget/widgets"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/safeish"
)

func New() (*application, error) {
	app := &application{}
	app.sys = wsi.NewSystem(app)
	return app, nil
}

type application struct {
	root          widget.Widget
	sys           *wsi.System
	win           *wsi.WaylandWindow
	widgetBinding *widget.Binding
	resized       bool
	size          wsi.LogicalSize

	renderer *sparse.Renderer
}

// WindowEvent implements wsi.Application.
func (app *application) WindowEvent(ctx *wsi.Context, ev wsi.Event) {
	switch ev := ev.(type) {
	case *wsi.EventInitialized:
		app.win = ctx.CreateWindow().(*wsi.WaylandWindow)
		app.widgetBinding = widget.RunApp(app.sys, app.win, app.root)
	case *wsi.Resized:
		if ev.Size == (wsi.LogicalSize{}) {
			ev.Size = wsi.LogicalSize{Width: 500, Height: 500}
		}
		sz := curve.Sz(float64(ev.Size.Width), float64(ev.Size.Height))
		app.widgetBinding.Renderer.View().SetConfiguration(render.ViewConfiguration{
			Min: sz,
			Max: sz,
		})
		app.size = ev.Size
		app.resized = true
		// log.Printf("setting logical size to %dx%d", ev.Size.Width, ev.Size.Height)
		app.win.SetSize(ev.Size)
		app.win.SetScale(ev.Scale)
	case *wsi.RedrawRequested:
		if app.resized {
			sz := app.win.PhysicalSize()
			app.renderer = sparse.NewRenderer(uint16(sz.Width), uint16(sz.Height))
			app.resized = false
		}

		const (
			printFrameTimes      = true
			printDetailedTimings = false
		)

		// XXX DPI scale and whatnot
		// OPT reuse recorders
		sz := app.win.PhysicalSize()
		// XXX make sure window size fits in uint16
		// log.Printf("rendering at physical size %dx%d", sz.Width, sz.Height)

		rec := gfx.NewRecorder()
		// XXX use actual scale, not 2
		rec.PushTransform(curve.Scale(2, 2))
		t := time.Now()
		startTime := t
		app.widgetBinding.DrawFrame(ev, rec.Checkpoint())
		if printDetailedTimings {
			log.Printf("recorded frame in: %s", time.Since(t))
		}

		t = time.Now()
		app.renderer.Reset()
		cmds := rec.Finish()
		if false {
			log.Println("---REPLAY START---")
			for _, cmd := range cmds {
				log.Printf("%#v", cmd)
			}
			log.Println("---REPLAY END---")
		}
		sparse.PlayRecording(cmds, app.renderer, curve.Identity)
		if printDetailedTimings {
			log.Printf("played recording in: %s", time.Since(t))
		}

		buf, err := app.win.NextBuffer()
		if err != nil {
			// XXX handle error more gracefully
			panic(err)
		}

		buff := safeish.SliceCast[[][4]uint8](buf.Data)

		t = time.Now()
		packer := &sparse.PackerUint8SRGB{
			Out:         buff,
			Width:       sz.Width,
			Height:      sz.Height,
			PremulAlpha: true,
		}
		app.renderer.Render(packer)
		if printDetailedTimings {
			log.Printf("rendered to pixmap in: %s", time.Since(t))
		}

		if printFrameTimes {
			log.Printf("rendered frame in: %s", time.Since(startTime))
		}

		app.win.Present(buf, 0, 0, sz.Width, sz.Height)
	case widgets.CallbackEvent:
		ev()
	default:
		panic(fmt.Sprintf("internal error: unhandled event type %T", ev))
	}
}

func (app *application) Run(ctx context.Context, root widget.Widget) error {
	app.root = root
	return app.sys.Run(ctx)
}

func init() {
	if gfx.ColorSpace != color.LinearSRGB {
		panic("expected linear SRGB values")
	}
}
