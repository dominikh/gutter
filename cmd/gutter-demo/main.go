package main

import (
	"image/color"
	"log"
	"os"
	"runtime/pprof"

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
	pprof.StartCPUProfile(os.Stdout)
	defer pprof.StopCPUProfile()
	r := render.NewRenderer()

	fc1 := &render.FillColor{}
	fc2 := &render.FillColor{}
	fc3 := &render.FillColor{}
	p := &render.Padding{}
	cons1 := &render.Constrained{}
	cons2 := &render.Constrained{}
	cons3 := &render.Constrained{}
	clip1 := &render.Clip{}
	clip2 := &render.Clip{}
	clip3 := &render.Clip{}
	row := &render.Row{}

	r.Register(fc1)
	r.Register(fc2)
	r.Register(fc3)
	r.Register(p)
	r.Register(row)
	r.Register(cons1)
	r.Register(cons2)
	r.Register(cons3)
	r.Register(clip1)
	r.Register(clip2)
	r.Register(clip3)

	fc1.SetColor(color.NRGBA{0xAB, 0x00, 0x00, 0xFF})
	fc2.SetColor(color.NRGBA{0xFF, 0x00, 0x00, 0xFF})
	fc3.SetColor(color.NRGBA{0x00, 0xFF, 0x00, 0xFF})

	p.SetInset(render.Inset{
		Left:   10,
		Top:    10,
		Right:  10,
		Bottom: 10,
	})
	cons1.SetExtraConstraints(render.Constraints{Min: f32.Pt(200, 100), Max: f32.Pt(200, 100)})
	cons2.SetExtraConstraints(render.Constraints{Min: f32.Pt(100, 100), Max: f32.Pt(100, 100)})
	cons3.SetExtraConstraints(render.Constraints{Min: f32.Pt(100, 100), Max: f32.Pt(100, 100)})
	clip1.SetChild(fc1)
	clip2.SetChild(fc2)
	clip3.SetChild(fc3)
	cons1.SetChild(clip1)
	cons2.SetChild(clip2)
	cons3.SetChild(clip3)
	row.AddChild(cons1)
	row.AddChild(cons2)
	row.AddChild(cons3)
	p.SetChild(row)

	g := p

	r.Initialize(g)

	N := 0
	var ops op.Ops
	for {
		switch e := w.NextEvent().(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			ops.Reset()

			// c := fc2.Color()
			// c.R++
			// fc2.SetColor(c)

			if N%1 == 0 {
				ins := p.Inset()
				ins.Left++
				p.SetInset(ins)
			}
			N++

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
