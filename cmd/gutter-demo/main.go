package main

import (
	"image/color"
	"log"
	"os"

	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/render"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
)

func main() {
	go func() {
		w := app.NewWindow()
		err := run(w)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	fc1 := &render.FillColor{}
	fc2 := &render.FillColor{}
	fc1.SetColor(color.NRGBA{0xFF, 0x00, 0x00, 0xFF})
	fc2.SetColor(color.NRGBA{0xFF, 0x00, 0x00, 0xFF})

	p := &render.Padding{}
	p.SetInset(render.Inset{
		Left:   10,
		Top:    10,
		Right:  10,
		Bottom: 10,
	})
	cons1 := &render.Constrained{}
	cons2 := &render.Constrained{}
	cons1.SetConstraints(render.Constraints{Min: f32.Pt(200, 100), Max: f32.Pt(200, 100)})
	cons2.SetConstraints(render.Constraints{Min: f32.Pt(100, 100), Max: f32.Pt(100, 100)})
	clip1 := &render.Clip{}
	clip2 := &render.Clip{}
	clip1.SetChild(fc1)
	clip2.SetChild(fc2)
	cons1.SetChild(clip1)
	cons2.SetChild(clip2)
	row := &render.Row{}
	row.AddChild(cons1)
	row.AddChild(cons2)
	p.SetChild(row)

	g := p

	r := render.NewRenderer()
	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			ops.Reset()

			c := fc1.Color()
			c.R++
			fc1.SetColor(c)
			fc1.MarkForRepaint()

			cs := render.Constraints{
				Min: f32.FPt(e.Size),
				Max: f32.FPt(e.Size),
			}
			r.Render(g, &ops, cs, f32.Point{})

			op.InvalidateOp{}.Add(&ops)
			e.Frame(&ops)
		}
	}
}
