package widget

import (
	"honnef.co/go/gutter/render"
)

var _ SingleChildWidget = (*Flexible)(nil)
var _ ParentDataWidget = (*Flexible)(nil)

var _ MultiChildRenderObjectWidget = (*Flex)(nil)

type MultiChildRenderObjectWidget interface {
	RenderObjectWidget
	MultiChildWidget
}

type Flex struct {
	Direction          render.Axis
	MainAxisAlignment  render.MainAxisAlignment
	MainAxisSize       render.MainAxisSize
	CrossAxisAlignment render.CrossAxisAlignment
	// XXX add clip behavior
	Children []Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (f *Flex) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.Flex{}
	f.UpdateRenderObject(ctx, obj)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (f *Flex) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	fobj := obj.(*render.Flex)
	fobj.SetDirection(f.Direction)
	fobj.SetMainAxisAlignment(f.MainAxisAlignment)
	fobj.SetMainAxisSize(f.MainAxisSize)
	fobj.SetCrossAxisAlignment(f.CrossAxisAlignment)
}

// CreateElement implements MultiChildWidget.
func (f *Flex) CreateElement() Element {
	return NewMultiChildRenderObjectElement(f)
}

// GetChild implements MultiChildWidget.
func (f *Flex) GetChildren() []Widget {
	return f.Children
}

type Flexible struct {
	Flex  int
	Fit   render.FlexFit
	Child Widget
}

// CreateElement implements SingleChildWidget.
func (f *Flexible) CreateElement() Element {
	return NewProxyElement(f)
}

// GetChild implements SingleChildWidget.
func (f *Flexible) GetChild() Widget {
	return f.Child
}

func (f *Flexible) ApplyParentData(obj render.Object) {
	data := obj.Handle().ParentData.(*render.FlexParentData)
	var needsLayout bool
	if data.Flex != f.Flex {
		data.Flex = f.Flex
		needsLayout = true
	}
	if data.Fit != f.Fit {
		data.Fit = f.Fit
		needsLayout = true
	}

	if needsLayout {
		if p := obj.Handle().Parent; p != nil {
			render.MarkNeedsLayout(p)
		}
	}
}
