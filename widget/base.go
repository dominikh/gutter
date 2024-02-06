package widget

import (
	"slices"
	"unsafe"

	"honnef.co/go/gutter/render"
)

var _ Element = (*ElementMixin)(nil)
var _ Element = (*ComponentElementMixin)(nil)
var _ Element = (*StatelessElementMixin)(nil)
var _ Element = (*RenderObjectElementMixin)(nil)
var _ Element = (*SingleChildRenderObjectElementMixin)(nil)
var _ RenderObjectElement = (*RenderObjectElementMixin)(nil)
var _ RenderObjectElement = (*SingleChildRenderObjectElementMixin)(nil)
var _ SingleChildElement = (*ComponentElementMixin)(nil)
var _ SingleChildElement = (*SingleChildRenderObjectElementMixin)(nil)

// TODO implement support for stateful widgets

// TODO MediaQuery
// TODO support inheritance (cf inheritedElements in framework.dart)
// TODO support "Notification"
// TODO support global keys

type BuildContext interface{}

type Widget interface {
	Key() any

	CreateElement() Element
}

type StatelessWidget interface {
	Widget
	Build(ctx BuildContext) Widget
}

type SingleChildWidget interface {
	Widget
	GetChild() Widget
}

type RenderObjectWidget interface {
	Widget
	CreateRenderObject(ctx BuildContext) render.Object
	UpdateRenderObject(ctx BuildContext, obj render.Object)
}

type Element interface {
	UpdateChild(child Element, newWidget Widget, newSlot any) Element
	Update(newWidget Widget)
	Handle() *ElementHandle
	Parent() Element
	Slot() any
	Activate()
	Deactivate()
	Mount(parent Element, newSlot any)
	Unmount()
	AttachRenderObject(slot any)
	DetachRenderObject()
	PerformRebuild()
	RenderObjectAttachingChild() Element
	UpdateSlot(newSlot any)
	VisitChildren(yield func(el Element) bool)
}

type SingleChildElement interface {
	Element
	GetChild() Element
	SetChild(child Element)
}

type RenderObjectElement interface {
	Element

	AncestorRenderObjectElement() RenderObjectElement
	RenderHandle() *RenderObjectElementHandle

	InsertRenderObjectChild(child render.Object, slot any)
	RemoveRenderObjectChild(child render.Object, slot any)
	MoveRenderObjectChild(child render.Object, oldSlot, newSlot any)
}

type ChildForgetter interface {
	ForgetChild(child Element)
}

type ElementMixin struct {
	ElementHandle
	Self Element
}

func (el *ElementMixin) Slot() any                                 { return el.slot }
func (el *ElementMixin) Parent() Element                           { return el.parent }
func (el *ElementMixin) VisitChildren(yield func(el Element) bool) {}
func (el *ElementMixin) UpdateSlot(newSlot any)                    { el.slot = newSlot }
func (el *ElementMixin) PerformRebuild()                           { el.dirty = false }

func (el *ElementMixin) MarkNeedsBuild() {
	if el.lifecycleState != ElementLifecycleActive {
		return
	}
	if el.dirty {
		return
	}
	el.dirty = true
	el.owner.scheduleBuildFor(el.Self)
}

// Activate implements Element.
func (el *ElementMixin) Activate() {
	// hadDependencies := (el._dependencies != null && el._dependencies.isNotEmpty) || el._hadUnsatisfiedDependencies // XXX implement once we have InheritedWidget
	el.lifecycleState = ElementLifecycleActive
	// We unregistered our dependencies in deactivate, but never cleared the list.
	// Since we're going to be reused, let's clear our list now.
	// XXX
	// if el._dependencies != nil {
	// 	el._dependencies.clear()
	// }
	// el._hadUnsatisfiedDependencies = false
	// el._updateInheritance()
	// el.attachNotificationTree()
	if el.dirty {
		el.owner.scheduleBuildFor(el.Self)
	}
	// if hadDependencies {
	// 	el.didChangeDependencies()
	// }
}

func (el *ElementMixin) Deactivate() {
	// XXX
	// if (_dependencies != null && _dependencies!.isNotEmpty) {
	//   for (final InheritedElement dependency in _dependencies!) {
	//     dependency.removeDependent(this);
	//   }
	//   // For expediency, we don't actually clear the list here, even though it's
	//   // no longer representative of what we are registered with. If we never
	//   // get re-used, it doesn't matter. If we do, then we'll clear the list in
	//   // activate(). The benefit of this is that it allows Element's activate()
	//   // implementation to decide whether to rebuild based on whether we had
	//   // dependencies here.
	// }
	// _inheritedElements = null;
	el.lifecycleState = ElementLifecycleInactive
}

// AttachRenderObject implements Element.
func (el *ElementMixin) AttachRenderObject(slot any) {
	el.Self.VisitChildren(func(child Element) bool {
		child.AttachRenderObject(slot)
		return true
	})
	el.slot = slot
}

// DetachRenderObject implements Element.
func (el *ElementMixin) DetachRenderObject() {
	el.Self.VisitChildren(func(child Element) bool {
		child.DetachRenderObject()
		return true
	})
	el.slot = nil
}

// Mount implements Element.
func (el *ElementMixin) Mount(parent Element, newSlot any) {
	el.parent = parent
	el.slot = newSlot
	el.lifecycleState = ElementLifecycleActive
	if parent != nil {
		el.depth = parent.Handle().depth
	} else {
		el.depth = 1
	}
	if parent != nil {
		// Only assign ownership if the parent is non-null. If parent is null
		// (the root node), the owner should have already been assigned.
		el.owner = parent.Handle().owner
	}
}

// RenderObjectAttachingChild implements Element.
func (el *ElementMixin) RenderObjectAttachingChild() Element {
	var out Element
	el.Self.VisitChildren(func(child Element) bool {
		out = child
		return false
	})
	return out
}

// Unmount implements Element.
func (el *ElementMixin) Unmount() {
	key := el.widget.Key()
	if key, ok := key.(*GlobalKey); ok {
		_ = key
		// owner!._unregisterGlobalKey(key, this); // XXX
	}
	el.lifecycleState = ElementLifecycleDefunct
}

// Update implements Element.
func (el *ElementMixin) Update(newWidget Widget) {
	el.widget = newWidget
}

// UpdateChild implements Element.
func (el *ElementMixin) UpdateChild(child Element, newWidget Widget, newSlot any) Element {
	if newWidget == nil {
		if child != nil {
			deactivateChild(child)
		}
		return nil
	}

	var newChild Element
	if child != nil {
		if child.Handle().widget == newWidget {
			if child.Handle().slot != newSlot {
				updateSlotForChild(el.Self, child, newSlot)
			}
			newChild = child
		} else if canUpdate(child.Handle().widget, newWidget) {
			if child.Handle().slot != newSlot {
				updateSlotForChild(el.Self, child, newSlot)
			}
			child.Update(newWidget)
			newChild = child
		} else {
			deactivateChild(child)
			newChild = InflateWidget(el.Self, newWidget, newSlot)
		}
	} else {
		newChild = InflateWidget(el.Self, newWidget, newSlot)
	}

	return newChild
}

type ComponentElementMixin struct {
	ElementMixin
	child Element
}

func (el *ComponentElementMixin) GetChild() Element      { return el.child }
func (el *ComponentElementMixin) SetChild(child Element) { el.child = child }

type WidgetBuilder interface {
	Build() Widget
}

// PerformRebuild implements Element.
func (el *ComponentElementMixin) PerformRebuild() {
	built := el.Self.(WidgetBuilder).Build()
	// We delay marking the element as clean until after calling build() so
	// that attempts to markNeedsBuild() during build() will be ignored.
	el.ElementMixin.PerformRebuild()
	el.child = el.Self.UpdateChild(el.child, built, el.slot)
}

// Mount implements Element.
func (el *ComponentElementMixin) Mount(parent Element, newSlot any) {
	el.ElementMixin.Mount(parent, newSlot)
	rebuild(el.Self)
}

// RenderObjectAttachingChild implements Element.
func (el *ComponentElementMixin) RenderObjectAttachingChild() Element {
	return el.child
}

func (el *ComponentElementMixin) VisitChildren(yield func(el Element) bool) {
	if el.child != nil {
		yield(el.child)
	}
}

func (el *ComponentElementMixin) ForgetChild(child Element) {
	el.child = nil
}

type StatelessElementMixin struct {
	ComponentElementMixin
}

// Update implements Element.
func (el *StatelessElementMixin) Update(newWidget Widget) {
	el.ComponentElementMixin.Update(newWidget)
	forceRebuild(el.Self)
}

type RenderObjectElementMixin struct {
	ElementMixin
	RenderObjectElementHandle
}

func (el *RenderObjectElementMixin) RenderObject() render.Object {
	return el.renderObject
}

func (el *RenderObjectElementMixin) AncestorRenderObjectElement() RenderObjectElement {
	return el.ancestorRenderObjectElement
}

// InsertRenderObjectChild implements RenderObjectElement.
func (*RenderObjectElementMixin) InsertRenderObjectChild(child render.Object, slot any) {
	panic("unimplemented")
}

// MoveRenderObjectChild implements RenderObjectElement.
func (*RenderObjectElementMixin) MoveRenderObjectChild(child render.Object, oldSlot any, newSlot any) {
	panic("unimplemented")
}

// RemoveRenderObjectChild implements RenderObjectElement.
func (*RenderObjectElementMixin) RemoveRenderObjectChild(child render.Object, slot any) {
	panic("unimplemented")
}

// UpdateSlot implements Element.
func (el *RenderObjectElementMixin) UpdateSlot(newSlot any) {
	oldSlot := el.slot
	el.ElementMixin.UpdateSlot(newSlot)
	if ancestor := el.ancestorRenderObjectElement; ancestor != nil {
		ancestor.MoveRenderObjectChild(el.renderObject, oldSlot, el.slot)
	}
}

// PerformRebuild implements Element.
func (el *RenderObjectElementMixin) PerformRebuild() {
	el.widget.(RenderObjectWidget).UpdateRenderObject(el.Self, el.renderObject)
	el.ElementMixin.PerformRebuild()
}

// AttachRenderObject implements Element.
func (el *RenderObjectElementMixin) AttachRenderObject(slot any) {
	el.slot = slot
	el.ancestorRenderObjectElement = findAncestorRenderObjectElement(el.Self.(RenderObjectElement))
	if el.ancestorRenderObjectElement != nil {
		el.ancestorRenderObjectElement.InsertRenderObjectChild(el.renderObject, slot)
	}
}

// DetachRenderObject implements Element.
func (el *RenderObjectElementMixin) DetachRenderObject() {
	if el.ancestorRenderObjectElement != nil {
		el.ancestorRenderObjectElement.RemoveRenderObjectChild(el.renderObject, el.slot)
		el.ancestorRenderObjectElement = nil
	}
	el.slot = nil
}

// Mount implements Element.
func (el *RenderObjectElementMixin) Mount(parent Element, newSlot any) {
	el.ElementMixin.Mount(parent, newSlot)

	el.renderObject = el.widget.(RenderObjectWidget).CreateRenderObject(el.Self)
	el.Self.(RenderObjectElement).AttachRenderObject(newSlot)
	el.Self.(RenderObjectElement).PerformRebuild() // clears the "dirty" flag
}

// RenderObjectAttachingChild implements Element.
func (*RenderObjectElementMixin) RenderObjectAttachingChild() Element {
	return nil
}

// Unmount implements Element.
func (el *RenderObjectElementMixin) Unmount() {
	oldWidget := el.widget.(RenderObjectWidget)
	el.ElementMixin.Unmount()
	if n, ok := oldWidget.(RenderObjectUnmountNotifyee); ok {
		n.DidUnmountRenderObject(el.renderObject)
	}
	render.Dispose(el.renderObject)
	el.renderObject = nil
}

// Update implements Element.
func (el *RenderObjectElementMixin) Update(newWidget Widget) {
	el.ElementMixin.Update(newWidget)
	el.Self.PerformRebuild()
}

type SingleChildRenderObjectElementMixin struct {
	RenderObjectElementMixin
	child Element
}

// Mount implements Element.
func (el *SingleChildRenderObjectElementMixin) Mount(parent Element, newSlot any) {
	el.RenderObjectElementMixin.Mount(parent, newSlot)
	el.child = el.Self.UpdateChild(el.child, el.widget.(SingleChildWidget).GetChild(), nil)
}

// Update implements Element.
func (el *SingleChildRenderObjectElementMixin) Update(newWidget Widget) {
	el.RenderObjectElementMixin.Update(newWidget)
	{
		self := el.Self.(SingleChildElement)
		self.SetChild(self.UpdateChild(self.GetChild(), el.widget.(SingleChildWidget).GetChild(), nil))
	}
}

func (el *SingleChildRenderObjectElementMixin) GetChild() Element         { return el.child }
func (el *SingleChildRenderObjectElementMixin) SetChild(child Element)    { el.child = child }
func (el *SingleChildRenderObjectElementMixin) ForgetChild(child Element) { el.child = nil }

func (s *SingleChildRenderObjectElementMixin) VisitChildren(yield func(el Element) bool) {
	if s.child != nil {
		yield(s.child)
	}
}

func (el *SingleChildRenderObjectElementMixin) InsertRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(child)
}

func (el *SingleChildRenderObjectElementMixin) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
	panic("unexpected call")
}

type RenderTreeRootElementMixin struct {
	RenderObjectElementMixin
}

func (el *RenderTreeRootElementMixin) AttachRenderObject(newSlot any) { el.slot = newSlot }
func (el *RenderTreeRootElementMixin) DetachRenderObject()            { el.slot = nil }

// XXX rename this
type BuildOwner struct {
	dirtyElements               []Element
	inactiveElements            inactiveElements
	dirtyElementsNeedsResorting bool
	onBuildScheduled            func()
	scheduledFlushDirtyElements bool
}

func (o *BuildOwner) scheduleBuildFor(el Element) {
	if el.Handle().inDirtyList {
		o.dirtyElementsNeedsResorting = true
		return
	}
	if !o.scheduledFlushDirtyElements && o.onBuildScheduled != nil {
		o.scheduledFlushDirtyElements = true
		o.onBuildScheduled()
	}
	o.dirtyElements = append(o.dirtyElements, el)
	el.Handle().inDirtyList = true
}

func (o *BuildOwner) BuildScope(context Element, callback func()) {
	if callback == nil && len(o.dirtyElements) == 0 {
		return
	}
	o.scheduledFlushDirtyElements = true
	if callback != nil {
		o.dirtyElementsNeedsResorting = false
		callback()
	}
	sortElements(o.dirtyElements)
	o.dirtyElementsNeedsResorting = false
	dirtyCount := len(o.dirtyElements)
	index := 0
	for index < dirtyCount {
		element := o.dirtyElements[index]
		rebuild(element)
		index += 1
		if dirtyCount < len(o.dirtyElements) || o.dirtyElementsNeedsResorting {
			sortElements(o.dirtyElements)
			o.dirtyElementsNeedsResorting = false
			dirtyCount = len(o.dirtyElements)
			for index > 0 && o.dirtyElements[index-1].Handle().dirty {
				// It is possible for previously dirty but inactive widgets to move right in the list.
				// We therefore have to move the index left in the list to account for this.
				// We don't know how many could have moved. However, we do know that the only possible
				// change to the list is that nodes that were previously to the left of the index have
				// now moved to be to the right of the right-most cleaned node, and we do know that
				// all the clean nodes were to the left of the index. So we move the index left
				// until just after the right-most clean node.
				index--
			}
		}
	}
	for _, element := range o.dirtyElements {
		element.Handle().inDirtyList = false
	}
	clear(o.dirtyElements)
	o.dirtyElements = o.dirtyElements[:0]
	o.scheduledFlushDirtyElements = false
	o.dirtyElementsNeedsResorting = false
}

func (o *BuildOwner) FinalizeTree() {
	o.inactiveElements.unmountAll()
}

const (
	ElementLifecycleIdle = iota
	ElementLifecycleActive
	ElementLifecycleInactive
	ElementLifecycleDefunct
)

type ElementHandle struct {
	parent         Element
	slot           any
	lifecycleState int
	depth          int
	owner          *BuildOwner
	dirty          bool
	inDirtyList    bool
	widget         Widget
}

func (el *ElementHandle) Handle() *ElementHandle { return el }

type RenderObjectElementHandle struct {
	renderObject                render.Object
	ancestorRenderObjectElement RenderObjectElement
}

func (el *RenderObjectElementHandle) RenderHandle() *RenderObjectElementHandle {
	return el
}

type RenderObjectUnmountNotifyee interface {
	DidUnmountRenderObject(obj render.Object)
}

func findAncestorRenderObjectElement(el RenderObjectElement) RenderObjectElement {
	ancestor := el.Parent()
	for ancestor != nil {
		if _, ok := ancestor.(RenderObjectElement); ok {
			break
		}
		ancestor = ancestor.Parent()
	}
	if ancestor == nil {
		return nil
	}
	return ancestor.(RenderObjectElement)
}

func sameType(a, b any) bool {
	return *(*unsafe.Pointer)(unsafe.Pointer(&a)) == *(*unsafe.Pointer)(unsafe.Pointer(&b))
}

func canUpdate(old, new Widget) bool {
	return sameType(old, new) && old.Key() == new.Key()
}

func deactivateChild(child Element) {
	child.Handle().parent = nil
	child.DetachRenderObject()
	child.Handle().owner.inactiveElements.add(child)
}

type inactiveElements struct {
	elements map[Element]struct{}
	locked   bool
}

func (els *inactiveElements) unmount(el Element) {
	el.VisitChildren(func(child Element) bool {
		els.unmount(child)
		return true
	})
	el.Unmount()
}

func (els *inactiveElements) unmountAll() {
	els.locked = true
	elements := make([]Element, 0, len(els.elements))
	for el := range els.elements {
		elements = append(elements, el)
	}
	sortElements(elements)
	clear(els.elements)
	for i := len(elements) - 1; i >= 0; i-- {
		els.unmount(elements[i])
	}
	els.locked = false
}

func sortElements(els []Element) {
	slices.SortFunc(els, func(a, b Element) int {
		ah := a.Handle()
		bh := b.Handle()
		diff := ah.depth - bh.depth
		// If depths are not equal, return the difference.
		if diff != 0 {
			return diff
		}
		// If the dirty values are not equal, sort with non-dirty elements being
		// less than dirty elements.
		isBDirty := bh.dirty
		if ah.dirty != isBDirty {
			if isBDirty {
				return -1
			} else {
				return 1
			}
		}
		// Otherwise, depths and dirtys are equal.
		return 0
	})
}

func (els *inactiveElements) deactivateRecursively(el Element) {
	el.Deactivate()
	el.VisitChildren(func(el Element) bool {
		els.deactivateRecursively(el)
		return true
	})
}

func (els *inactiveElements) add(el Element) {
	if el.Handle().lifecycleState == ElementLifecycleActive {
		els.deactivateRecursively(el)
	}
	els.elements[el] = struct{}{}
}

func (els *inactiveElements) remove(el Element) {
	delete(els.elements, el)
}

// Key -> GlobalKey -> GlobalObjectKey

// Use values, not pointers.
type GlobalObjectKey struct {
	object any
}

type GlobalKey struct {
	Value any
}

func (g GlobalKey) currentElement() Element {
	// XXX implement
	return nil
}

func InflateWidget(parent Element, widget Widget, slot any) Element {
	key := widget.Key()
	if key, ok := key.(GlobalKey); ok {
		newChild := RetakeInactiveElement(parent, key, widget)
		if newChild != nil {
			activateWithParent(newChild, parent, slot)
			updatedChild := parent.UpdateChild(newChild, widget, slot)
			return updatedChild
		}
	}
	newChild := widget.CreateElement()
	newChild.Mount(parent, slot)

	return newChild
}

func RetakeInactiveElement(el Element, key GlobalKey, newWidget Widget) Element {
	// The "inactivity" of the element being retaken here may be forward-looking: if
	// we are taking an element with a GlobalKey from an element that currently has
	// it as a child, then we know that element will soon no longer have that
	// element as a child. The only way that assumption could be false is if the
	// global key is being duplicated, and we'll try to track that using the
	// _debugTrackElementThatWillNeedToBeRebuiltDueToGlobalKeyShenanigans call below.
	element := key.currentElement()
	if element == nil {
		return nil
	}
	if !canUpdate(element.Handle().widget, newWidget) {
		return nil
	}
	parent := element.Parent()
	if parent != nil {
		if parent, ok := parent.(ChildForgetter); ok {
			parent.ForgetChild(element)
		}
		deactivateChild(element)
	}
	el.Handle().owner.inactiveElements.remove(element)
	return element
}

func activateWithParent(el, parent Element, newSlot any) {
	el.Handle().parent = parent
	updateDepth(el, parent.Handle().depth)
	activateRecursively(el)
	el.AttachRenderObject(newSlot)
}

func updateDepth(el Element, parentDepth int) {
	expectedDepth := parentDepth + 1
	if el.Handle().depth < expectedDepth {
		el.Handle().depth = expectedDepth
		el.VisitChildren(func(child Element) bool {
			updateDepth(child, expectedDepth)
			return true
		})
	}
}

func activateRecursively(element Element) {
	element.Activate()
	element.VisitChildren(func(child Element) bool {
		activateRecursively(child)
		return true
	})
}

func updateSlotForChild(el, child Element, newSlot any) {
	var visit func(element Element)
	visit = func(element Element) {
		element.UpdateSlot(newSlot)
		descendant := element.RenderObjectAttachingChild()
		if descendant != nil {
			visit(descendant)
		}
	}
	visit(child)
}

func rebuild(el Element) {
	if el.Handle().lifecycleState != ElementLifecycleActive || !el.Handle().dirty {
		return
	}
	el.PerformRebuild()
}

func forceRebuild(el Element) {
	if el.Handle().lifecycleState != ElementLifecycleActive {
		return
	}
	el.PerformRebuild()
}
