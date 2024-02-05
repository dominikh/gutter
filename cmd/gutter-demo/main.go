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

func run2(w *app.Window) error {
	var root widget.Widget = &widget.Padding{
		Padding: render.Inset{20, 20, 20, 20},
		// XXX This SizedBox doesn't actually do anything, because we never loosen the constraints.
		ChildWidget: &widget.SizedBox{
			Width:  50,
			Height: 50,
			ChildWidget: &widget.ColoredBox{
				Color: color.NRGBA{255, 0, 0, 255},
			},
		},
	}

	// This is basically runApp
	po := render.NewPipelineOwner()
	wview := widget.NewView(root, po)
	var bo widget.BuildOwner
	rootElem := (&widget.RootWidget{wview}).Attach(&bo, nil)

	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			// wview.DrawFrame(e)

			// cs := render.Constraints{
			// 	Min: f32.FPt(e.Size),
			// 	Max: f32.FPt(e.Size),
			// }

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
