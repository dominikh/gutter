package widget

import (
	"log"

	"gioui.org/op"
	"honnef.co/go/gutter/render"
)

type WidgetsBinding struct {
	buildOwner  BuildOwner
	rootElement *RootElement

	rendererBinding *render.RendererBinding
}

// XXX Flutter uses a singleton… we really don't want to
var TheWidgetsBinding = NewWidgetsBinding()

func NewWidgetsBinding() *WidgetsBinding {
	b := WidgetsBinding{}
	b.buildOwner.onBuildScheduled = func() {
		log.Println("!!! onBuildScheduled")
	}
	b.rendererBinding = render.TheRendererBinding
	return &b
}

func (wb *WidgetsBinding) DrawFrame(ops *op.Ops) {
	if wb.rootElement != nil {
		wb.buildOwner.BuildScope(wb.rootElement, nil)
	}
	wb.rendererBinding.DrawFrame(ops)
	wb.buildOwner.FinalizeTree()
}

func (wb *WidgetsBinding) AttachRootWidget(rootWidget Widget) {
	wb.attachToBuildOwner(&RootWidget{Child: rootWidget})
}

// / Called by [attachRootWidget] to attach the provided [RootWidget] to the
// / [buildOwner].
// /
// / This creates the [rootElement], if necessary, or re-uses an existing one.
// /
// / This method is rarely called directly, but it can be useful in tests to
// / restore the element tree to a previous version by providing the
// / [RootWidget] of that version (see [WidgetTester.restartAndRestore] for an
// / exemplary use case).
func (wb *WidgetsBinding) attachToBuildOwner(widget *RootWidget) {
	// isBootstrapFrame := wb.rootElement == nil
	// _readyToProduceFrames = true;
	wb.rootElement = widget.Attach(&wb.buildOwner, wb.rootElement)
}

func (wb *WidgetsBinding) isRootWidgetAttached() bool {
	return wb.rootElement != nil
}

// / Used by [runApp] to wrap the provided `rootWidget` in the default [View].
// /
// / The [View] determines into what [FlutterView] the app is rendered into.
// / This is currently [PlatformDispatcher.implicitView] from [platformDispatcher].
// /
// / The `rootWidget` widget provided to this method must not already be
// / wrapped in a [View].
func (wb *WidgetsBinding) WrapWithDefaultView(rootWidget Widget) Widget {
	return &RawView{
		// view:    nil, // XXX
		Child: rootWidget,
		builder: func(ctx BuildContext, owner *render.PipelineOwner) Widget {
			return rootWidget
		},
	}
}
