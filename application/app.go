// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package application

import (
	"fmt"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/gutter/widget/widgets"
	"honnef.co/go/gutter/wsi"
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
		app.win.SetSize(ev.Size)
		app.win.SetScale(1)
	case *wsi.RedrawRequested:
		if app.resized {
		}

		// XXX
		app.widgetBinding.DrawFrame(ev, nil)
	case widgets.CallbackEvent:
	default:
		panic(fmt.Sprintf("internal error: unhandled event type %T", ev))
	}
}
