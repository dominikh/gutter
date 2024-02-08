package widget

import (
	"slices"
	"unsafe"

	"honnef.co/go/gutter/render"
)

type ElementTransitionKind uint8

const (
	ElementMounted ElementTransitionKind = iota
	ElementChangedDependencies
	ElementUpdated
	ElementDeactivating
	ElementActivated
	ElementUnmounted
)

type StateTransitionKind uint8

const (
	StateInitializing StateTransitionKind = iota
	StateUpdatedWidget
	StateChangedDependencies
	StateDeactivating
	StateActivating
	StateDisposing
)

type ElementTransition struct {
	Kind ElementTransitionKind

	// The new widget for Kind == ElementUpdated
	NewWidget Widget

	// The parent and slot for Kind == ElementMounted
	Parent  Element
	NewSlot any
}

type StateTransition struct {
	Kind StateTransitionKind

	// The old widget for Kind == StateUpdatedWidget
	OldWidget StatefulWidget
}

// TODO MediaQuery
// TODO support inheritance (cf inheritedElements in framework.dart)
// TODO support "Notification"
// TODO support global keys

func NewInteriorElement(w Widget) InteriorElement {
	se := &SimpleInteriorElement{}
	se.ElementHandle.widget = w
	if w, ok := w.(StatefulWidget); ok {
		se.State = w.CreateState()
		sh := se.State.GetStateHandle()
		sh.Widget = w
		sh.Element = se
	}
	return se
}

var _ InteriorElement = (*SimpleInteriorElement)(nil)
var _ WidgetBuilder = (*SimpleInteriorElement)(nil)

type SimpleInteriorElement struct {
	ElementHandle
	State

	child Element
}

func (el *SimpleInteriorElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementMounted:
		if s := el.State; s != nil {
			s.Transition(StateTransition{Kind: StateInitializing})
			s.Transition(StateTransition{Kind: StateChangedDependencies})
		}
		rebuild(el)
	case ElementActivated:
		if s := el.State; s != nil {
			s.Transition(StateTransition{Kind: StateActivating})
		}
		MarkNeedsBuild(el)
	case ElementUnmounted:
		if s := el.State; s != nil {
			h := s.GetStateHandle()
			s.Transition(StateTransition{Kind: StateDisposing})
			h.Element = nil
		}
	case ElementUpdated:
		if s := el.State; s != nil {
			h := el.GetStateHandle()
			oldWidget := h.Widget
			h.Widget = el.Handle().widget
			s.Transition(StateTransition{Kind: StateUpdatedWidget, OldWidget: oldWidget.(StatefulWidget)})
		}
		forceRebuild(el)
	case ElementDeactivating:
		if s := el.State; s != nil {
			s.Transition(StateTransition{Kind: StateDeactivating})
		}
	case ElementChangedDependencies:
		if s := el.State; s != nil {
			el.GetStateHandle().didChangeDependencies = true
		}
	}
}

func (el *SimpleInteriorElement) GetChild() Element {
	return el.child
}

func (el *SimpleInteriorElement) SetChild(child Element) {
	el.child = child
}

func (el *SimpleInteriorElement) GetState() State {
	return el.State
}

func (el *SimpleInteriorElement) Build() Widget {
	if s := el.State; s != nil {
		return s.Build()
	} else {
		return el.widget.(WidgetBuilder).Build()
	}
}

func (el *SimpleInteriorElement) PerformRebuild() {
	if s := el.State; s != nil {
		h := el.GetStateHandle()
		if h.didChangeDependencies {
			s.Transition(StateTransition{Kind: StateChangedDependencies})
			h.didChangeDependencies = false
		}
	}
	built := el.Build()
	el.SetChild(UpdateChild(el, el.GetChild(), built, el.Handle().slot))
	el.Handle().dirty = false
}

type BuildContext interface{}

type Widget interface {
	Key() any

	CreateElement() Element
}

type StatelessWidget interface {
	Widget
	Build(ctx BuildContext) Widget
}

type StatefulWidget interface {
	Widget
	CreateState() State
}

// State is state.
//
// Implementations can optionally implement [InitStater], [StateDidUpdateWidgeter], [StateDeactivater],
// [StateActivater], [Disposer], and [StateDidChangeDependencieser].
type State interface {
	WidgetBuilder

	GetStateHandle() *StateHandle
	Transition(t StateTransition)
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
	Handle() *ElementHandle
	Transition(t ElementTransition)
	PerformRebuild()
}

type InteriorElement interface {
	Element

	SingleChildElement
	WidgetBuilder
	GetState() State
}

func DidChangeDependencies(el Element) {
	MarkNeedsBuild(el)
	el.Transition(ElementTransition{Kind: ElementChangedDependencies})
}

func Update(el Element, newWidget Widget) {
	el.Handle().widget = newWidget
	el.Transition(ElementTransition{Kind: ElementUpdated, NewWidget: newWidget})
}

func RenderObjectAttachingChild(el Element) Element {
	// XXX does this work correctlyf or SingleChildRenderObjectElement? RenderObjectElement used to override
	// AttachingChild to return nil. It doesn't have to anymore, because this function can't find any
	// children. But what about SingleChildRenderObjectElement? That does have children.
	var out Element
	VisitChildren(el, func(child Element) bool {
		out = child
		return false
	})
	return out
}

func MarkNeedsBuild(el Element) {
	h := el.Handle()
	if h.lifecycleState != ElementLifecycleActive {
		return
	}
	if h.dirty {
		return
	}
	h.dirty = true
	h.owner.scheduleBuildFor(el)
}

type RenderObjectAttacher interface {
	AttachRenderObject(slot any)
}

func AttachRenderObject(el Element, slot any) {
	if el, ok := el.(RenderObjectAttacher); ok {
		el.AttachRenderObject(slot)
		return
	}
	VisitChildren(el, func(child Element) bool {
		AttachRenderObject(child, slot)
		return true
	})
	el.Handle().slot = slot
}

type RenderObjectDetacher interface {
	AfterDetachRenderObject()
}

// DetachRenderObject recursively instructs the children of the element to detach their render object.
// Elements that implement RenderObjectDetacher additionally have their AfterDetachRenderObject method called.
func DetachRenderObject(el Element) {
	VisitChildren(el, func(child Element) bool {
		DetachRenderObject(child)
		return true
	})
	el.Handle().slot = nil
	if el, ok := el.(RenderObjectDetacher); ok {
		el.AfterDetachRenderObject()
	}
}

type SlotUpdater interface {
	AfterUpdateSlot(oldSlot, newSlot any)
}

// UpdateSlot updates the element's slot. If the element implements SlotUpdater, AfterUpdateSlot is called
// afterwards with the old and new slots.
func UpdateSlot(el Element, newSlot any) {
	h := el.Handle()
	old := h.slot
	h.slot = newSlot
	if el, ok := el.(SlotUpdater); ok {
		el.AfterUpdateSlot(old, newSlot)
	}
}

func UpdateChild(el, child Element, newWidget Widget, newSlot any) Element {
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
				updateSlotForChild(el, child, newSlot)
			}
			newChild = child
		} else if canUpdate(child.Handle().widget, newWidget) {
			if child.Handle().slot != newSlot {
				updateSlotForChild(el, child, newSlot)
			}
			Update(child, newWidget)
			newChild = child
		} else {
			deactivateChild(child)
			newChild = InflateWidget(el, newWidget, newSlot)
		}
	} else {
		newChild = InflateWidget(el, newWidget, newSlot)
	}

	return newChild
}

// Activate activates the element. If it implements Activater, the AfterActivate method will be called afterwards.
func Activate(el Element) {
	// hadDependencies := (el._dependencies != null && el._dependencies.isNotEmpty) || el._hadUnsatisfiedDependencies // XXX implement once we have InheritedWidget

	h := el.Handle()
	h.lifecycleState = ElementLifecycleActive
	// We unregistered our dependencies in deactivate, but never cleared the list.
	// Since we're going to be reused, let's clear our list now.
	// XXX
	// if el._dependencies != nil {
	// 	el._dependencies.clear()
	// }
	// el._hadUnsatisfiedDependencies = false
	// el._updateInheritance()
	// el.attachNotificationTree()
	if h.dirty {
		h.owner.scheduleBuildFor(el)
	}
	// if hadDependencies {
	// 	el.didChangeDependencies()
	// }

	el.Transition(ElementTransition{Kind: ElementActivated})
}

func Deactivate(el Element) {
	el.Transition(ElementTransition{Kind: ElementDeactivating})

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
	el.Handle().lifecycleState = ElementLifecycleInactive
}

func Mount(el, parent Element, newSlot any) {
	h := el.Handle()
	h.parent = parent
	h.slot = newSlot
	h.lifecycleState = ElementLifecycleActive
	if parent != nil {
		h.depth = parent.Handle().depth
	} else {
		h.depth = 1
	}
	if parent != nil {
		// Only assign ownership if the parent is non-null. If parent is null
		// (the root node), the owner should have already been assigned.
		h.owner = parent.Handle().owner
	}

	el.Handle().dirty = true
	el.Transition(ElementTransition{Kind: ElementMounted, Parent: parent, NewSlot: newSlot})
}

// Unmount unmounts the element. If it implements Unmounter, its AfterUnmount method will be called afterwards.
func Unmount(el Element) {
	h := el.Handle()
	key := h.widget.Key()
	if key, ok := key.(*GlobalKey); ok {
		_ = key
		// owner!._unregisterGlobalKey(key, this); // XXX
	}
	h.lifecycleState = ElementLifecycleDefunct

	el.Transition(ElementTransition{Kind: ElementUnmounted})
}

type ChildrenVisiter interface {
	VisitChildren(yield func(el Element) bool)
}

// VisitChildren visits an element's children, by using its VisitChildren or GetChild methods.
func VisitChildren(el Element, yield func(Element) bool) {
	switch el := el.(type) {
	case ChildrenVisiter:
		el.VisitChildren(yield)
	case SingleChildElement:
		if child := el.GetChild(); child != nil {
			yield(child)
		}
	}
}

type ChildForgetter interface {
	ForgetChild(child Element)
}

// ForgetChild instructs an element to forget one of its children, either by calling ForgetChild on it or by
// calling SetChild(nil).
func ForgetChild(el Element, child Element) {
	switch el := el.(type) {
	case ChildForgetter:
		el.ForgetChild(child)
	case SingleChildElement:
		el.SetChild(nil)
	}
}

type SingleChildElement interface {
	Element
	GetChild() Element
	SetChild(child Element)
}

type WidgetBuilder interface {
	Build() Widget
}

var _ SingleChildRenderObjectElement = (*SimpleSingleChildRenderObjectElement)(nil)

type SimpleSingleChildRenderObjectElement struct {
	RenderObjectElementHandle
	child Element
}

// Transition implements SingleChildRenderObjectElement.
func (el *SimpleSingleChildRenderObjectElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementMounted:
		SingleChildRenderObjectElementAfterMount(el, t.Parent, t.NewSlot)
	case ElementUnmounted:
		SingleChildRenderObjectElementAfterUnmount(el)
	case ElementUpdated:
		SingleChildRenderObjectElementAfterUpdate(el, t.NewWidget)
	}
}

// AttachRenderObject implements SingleChildRenderObjectElement.
func (el *SimpleSingleChildRenderObjectElement) AttachRenderObject(slot any) {
	SingleChildRenderObjectElementAttachRenderObject(el, slot)
}

// PerformRebuild implements SingleChildRenderObjectElement.
func (el *SimpleSingleChildRenderObjectElement) PerformRebuild() {
	SingleChildRenderObjectElementPerformRebuild(el)
}

// RemoveRenderObjectChild implements SingleChildRenderObjectElement.
func (*SimpleSingleChildRenderObjectElement) RemoveRenderObjectChild(child render.Object, slot any) {
	panic("unimplemented")
}

func (el *SimpleSingleChildRenderObjectElement) GetChild() Element      { return el.child }
func (el *SimpleSingleChildRenderObjectElement) SetChild(child Element) { el.child = child }

func (el *SimpleSingleChildRenderObjectElement) InsertRenderObjectChild(child render.Object, slot any) {
	render.SetChild(el.renderObject.(render.ObjectWithChild), child)
}

func (el *SimpleSingleChildRenderObjectElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
	panic("unexpected call")
}

// XXX rename this
type BuildOwner struct {
	dirtyElements               []Element
	inactiveElements            inactiveElements
	dirtyElementsNeedsResorting bool
	OnBuildScheduled            func()
	scheduledFlushDirtyElements bool
}

func (o *BuildOwner) scheduleBuildFor(el Element) {
	if el.Handle().inDirtyList {
		o.dirtyElementsNeedsResorting = true
		return
	}
	if !o.scheduledFlushDirtyElements && o.OnBuildScheduled != nil {
		o.scheduledFlushDirtyElements = true
		o.OnBuildScheduled()
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

type StateHandle struct {
	Widget                Widget
	Element               Element
	didChangeDependencies bool
}

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

func (h *StateHandle) GetStateHandle() *StateHandle { return h }

func (el *ElementHandle) Handle() *ElementHandle { return el }
func (el *ElementHandle) Parent() Element        { return el.parent }
func (el *ElementHandle) Slot() any              { return el.slot }

type RenderObjectElementHandle struct {
	ElementHandle
	renderObject                render.Object
	ancestorRenderObjectElement RenderObjectElement
}

func (el *RenderObjectElementHandle) RenderHandle() *RenderObjectElementHandle {
	return el
}

func (h *RenderObjectElementHandle) UpdateSlot(oldSlot, newSlot any) {
	if ancestor := h.ancestorRenderObjectElement; ancestor != nil {
		ancestor.MoveRenderObjectChild(h.renderObject, oldSlot, h.slot)
	}
}

func (el *RenderObjectElementHandle) DetachRenderObject() {
	if el.ancestorRenderObjectElement != nil {
		el.ancestorRenderObjectElement.RemoveRenderObjectChild(el.renderObject, el.slot)
		el.ancestorRenderObjectElement = nil
	}
	el.slot = nil
}

type RenderObjectUnmountNotifyee interface {
	DidUnmountRenderObject(obj render.Object)
}

func findAncestorRenderObjectElement(el RenderObjectElement) RenderObjectElement {
	ancestor := el.Handle().Parent()
	for ancestor != nil {
		if _, ok := ancestor.(RenderObjectElement); ok {
			break
		}
		ancestor = ancestor.Handle().Parent()
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
	DetachRenderObject(child)
	child.Handle().owner.inactiveElements.add(child)
}

type inactiveElements struct {
	elements map[Element]struct{}
	locked   bool
}

func (els *inactiveElements) unmount(el Element) {
	VisitChildren(el, func(child Element) bool {
		els.unmount(child)
		return true
	})
	Unmount(el)
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
	Deactivate(el)
	VisitChildren(el, func(el Element) bool {
		els.deactivateRecursively(el)
		return true
	})
}

func (els *inactiveElements) add(el Element) {
	if el.Handle().lifecycleState == ElementLifecycleActive {
		els.deactivateRecursively(el)
	}
	// OPT(dh): move this initialization to a constructor
	if els.elements == nil {
		els.elements = make(map[Element]struct{})
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
			updatedChild := UpdateChild(parent, newChild, widget, slot)
			return updatedChild
		}
	}
	newChild := widget.CreateElement()
	Mount(newChild, parent, slot)

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
	parent := element.Handle().Parent()
	if parent != nil {
		ForgetChild(parent, element)
		deactivateChild(element)
	}
	el.Handle().owner.inactiveElements.remove(element)
	return element
}

func activateWithParent(el, parent Element, newSlot any) {
	el.Handle().parent = parent
	updateDepth(el, parent.Handle().depth)
	activateRecursively(el)
	AttachRenderObject(el, newSlot)
}

func updateDepth(el Element, parentDepth int) {
	expectedDepth := parentDepth + 1
	if el.Handle().depth < expectedDepth {
		el.Handle().depth = expectedDepth
		VisitChildren(el, func(child Element) bool {
			updateDepth(child, expectedDepth)
			return true
		})
	}
}

func activateRecursively(element Element) {
	Activate(element)
	VisitChildren(element, func(child Element) bool {
		activateRecursively(child)
		return true
	})
}

func updateSlotForChild(el, child Element, newSlot any) {
	var visit func(element Element)
	visit = func(element Element) {
		UpdateSlot(element, newSlot)
		descendant := RenderObjectAttachingChild(element)
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
	if el, ok := el.(Rebuilder); ok {
		el.PerformRebuild()
	}
	el.Handle().dirty = false
}

func forceRebuild(el Element) {
	if el.Handle().lifecycleState != ElementLifecycleActive {
		return
	}
	if el, ok := el.(Rebuilder); ok {
		el.PerformRebuild()
	}
	el.Handle().dirty = false
}

type Rebuilder interface {
	PerformRebuild()
}
