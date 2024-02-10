package render

import (
	"gioui.org/f32"
	"gioui.org/op"
)

var _ Object = (*View)(nil)

type View struct {
	ObjectHandle
	SingleChild

	r             *Renderer
	ops           op.Ops
	configuration ViewConfiguration
}

func NewView() *View {
	return &View{
		r: NewRenderer(),
	}
}

func (v *View) PerformPaint(r *Renderer, ops *op.Ops) {
	if v.Child != nil {
		r.Paint(v.Child).Add(ops)
	}
}

// XXX include pxperdp etc in the view configuration
type ViewConfiguration = Constraints

func (v *View) SetConfiguration(value ViewConfiguration) {
	if v.configuration == value {
		return
	}
	v.configuration = value
	MarkNeedsLayout(v)
}

func (v *View) PrepareInitialFrame() {
	ScheduleInitialLayout(v)
	ScheduleInitialPaint(v)
}

func (v *View) constraints() Constraints {
	return v.configuration
}

func (v *View) PerformLayout() f32.Point {
	sizedByChild := !v.constraints().Tight()
	if v.Child != nil {
		Layout(v.Child, v.constraints(), sizedByChild)
	}
	if sizedByChild && v.Child != nil {
		return v.Child.Handle().size
	} else {
		return v.constraints().Min
	}
}
