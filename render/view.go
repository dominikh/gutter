package render

import (
	"fmt"

	"gioui.org/f32"
	"gioui.org/op"
)

var _ Object = (*View)(nil)

type View struct {
	ObjectHandle
	SingleChild

	configuration ViewConfiguration
}

func (v *View) MarkNeedsPaint()  { MarkNeedsPaint(v) }
func (v *View) MarkNeedsLayout() { MarkNeedsLayout(v) }

func (v *View) Paint(r *Renderer, ops *op.Ops) {
	if v.Child != nil {
		fmt.Printf("ops in render.View.Paint: %p\n", ops)
		r.Paint(v.Child).Add(ops)
	}
}

// XXX include pxperdp etc in the view configuration
type ViewConfiguration = Constraints

func NewView(child Object, config ViewConfiguration /* , view *FlutterView */) *View {
	var v View
	v.SetChild(child)
	v.SetConfiguration(config)
	// v.view = view
	return &v
}

func (v *View) SetConfiguration(value ViewConfiguration) {
	if v.configuration == value {
		return
	}
	v.configuration = value
	// if _rootTransform == nil {
	// 	// [prepareInitialFrame] has not been called yet, nothing to do for now.
	// 	return
	// }
	// if (oldConfiguration?.toMatrix() != configuration.toMatrix()) {
	//   replaceRootLayer(_updateMatricesAndCreateNewRootLayer());
	// }
	v.MarkNeedsLayout()
}

func (v *View) PrepareInitialFrame() {
	ScheduleInitialLayout(v)
	ScheduleInitialPaint(v)
}

func (v *View) constraints() Constraints {
	return v.configuration
}

func (v *View) Layout() f32.Point {
	sizedByChild := !v.constraints().Tight()
	if v.Child != nil {
		// panic(v.constraints.Max.String())
		Layout(v.Child, v.constraints(), sizedByChild)
	}
	if sizedByChild && v.Child != nil {
		return v.Child.Handle().size
	} else {
		return v.constraints().Min
	}
}

// func (v *View) CompositeFrame() {
// 	builder := ui.SceneBuilder()
// 	scene := layer.buildScene(builder)
// 	v.view.render(scene, v.size)
// 	scene.dispose()
// }

/*
/// The root of the render tree.
///
/// The view represents the total output surface of the render tree and handles
/// bootstrapping the rendering pipeline. The view has a unique child
/// [RenderBox], which is required to fill the entire output surface.
class RenderView extends RenderObject with RenderObjectWithChildMixin<RenderBox> {

  /// The [FlutterView] into which this [RenderView] will render.
  ui.FlutterView get flutterView => _view;

  @override
  bool get isRepaintBoundary => true;
}
*/
