package widget

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"unsafe"

	"honnef.co/go/gutter/debug"
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

type RenderObjectElement interface {
	HasChildElement

	RenderHandle() *RenderObjectElementHandle

	InsertRenderObjectChild(child render.Object, slot int)
	RemoveRenderObjectChild(child render.Object, slot int)
	MoveRenderObjectChild(child render.Object, newSlot int)

	AttachRenderObject(slot int)
}

type ElementTransition struct {
	Kind ElementTransitionKind

	// The old widget for Kind == ElementUpdated
	OldWidget Widget

	// The parent and slot for Kind == ElementMounted
	Parent  Element
	NewSlot int
}

type StateTransition[W Widget] struct {
	Kind StateTransitionKind

	// The old widget for Kind == StateUpdatedWidget
	OldWidget W
}

// TODO MediaQuery
// TODO support inheritance (cf inheritedElements in framework.dart)
// TODO support "Notification"
// TODO support global keys

func NewProxyElement[W Widget](w W) InteriorElement {
	el := &ProxyElement{}
	el.widget = w
	return el
}

type ProxyElement struct {
	ElementHandle
	child Element
}

// Children implements InteriorElement.
func (el *ProxyElement) Children() []Element {
	if el.child == nil {
		return nil
	} else {
		// OPT(dh)
		return []Element{el.child}
	}
}

// ForgottenChildren implements InteriorElement.
func (el *ProxyElement) ForgottenChildren() map[Element]struct{} {
	return nil
}

// SetChildren implements InteriorElement.
func (el *ProxyElement) SetChildren(children []Element) {
	debug.Assert(len(children) < 2)
	if len(children) == 0 {
		el.child = nil
	} else {
		el.child = children[0]
	}
}

// VisitChildren implements InteriorElement.
func (el *ProxyElement) VisitChildren(yield func(e Element) bool) {
	if el.child == nil {
		return
	}
	yield(el.child)
}

// Build implements InteriorElement.
func (p *ProxyElement) Build() Widget {
	return GetWidgetChild(p.widget)
}

// GetChild implements InteriorElement.
func (p *ProxyElement) GetChild() Element {
	return p.child
}

// PerformRebuild implements InteriorElement.
func (el *ProxyElement) PerformRebuild() {
	built := el.Build()
	el.child = UpdateChild(el, el.GetChild(), built, el.Handle().slot)
	el.Handle().dirty = false
}

// Transition implements InteriorElement.
func (el *ProxyElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementMounted:
		rebuild(el)
	case ElementActivated:
		MarkNeedsBuild(el)
	case ElementUpdated:
		forceRebuild(el)
	}
}

func NewInteriorElement[W Widget](w W) InteriorElement {
	se := &SimpleInteriorElement[W]{}
	se.ElementHandle.widget = w
	if w2, ok := any(w).(StatefulWidget[W]); ok {
		se.State = w2.CreateState()
		sh := se.State.GetStateHandle()
		sh.Widget = w
		sh.Element = se
	}
	return se
}

type SimpleInteriorElement[W Widget] struct {
	ElementHandle
	State[W]

	child Element
}

// Children implements InteriorElement.
func (el *SimpleInteriorElement[W]) Children() []Element {
	// XXX ProxyElement, viewElement, InteriorElement all have methods in common

	if el.child == nil {
		return nil
	} else {
		// OPT(dh)
		return []Element{el.child}
	}
}

// ForgottenChildren implements InteriorElement.
func (el *SimpleInteriorElement[W]) ForgottenChildren() map[Element]struct{} {
	return nil
}

// SetChildren implements InteriorElement.
func (el *SimpleInteriorElement[W]) SetChildren(children []Element) {
	debug.Assert(len(children) < 2)
	if len(children) == 0 {
		el.child = nil
	} else {
		el.child = children[0]
	}
}

// VisitChildren implements InteriorElement.
func (el *SimpleInteriorElement[W]) VisitChildren(yield func(e Element) bool) {
	if el.child == nil {
		return
	}
	yield(el.child)
}

func (el *SimpleInteriorElement[W]) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementMounted:
		if s := el.State; s != nil {
			s.Transition(StateTransition[W]{Kind: StateInitializing})
			s.Transition(StateTransition[W]{Kind: StateChangedDependencies})
		}
		rebuild(el)
	case ElementActivated:
		if s := el.State; s != nil {
			s.Transition(StateTransition[W]{Kind: StateActivating})
		}
		MarkNeedsBuild(el)
	case ElementUnmounted:
		if s := el.State; s != nil {
			h := s.GetStateHandle()
			s.Transition(StateTransition[W]{Kind: StateDisposing})
			h.Element = nil
		}
	case ElementUpdated:
		if s := el.State; s != nil {
			h := el.GetStateHandle()
			oldWidget := h.Widget
			h.Widget = el.Handle().widget.(W)
			s.Transition(StateTransition[W]{Kind: StateUpdatedWidget, OldWidget: oldWidget})
		}
		forceRebuild(el)
	case ElementDeactivating:
		if s := el.State; s != nil {
			s.Transition(StateTransition[W]{Kind: StateDeactivating})
		}
	case ElementChangedDependencies:
		if s := el.State; s != nil {
			el.GetStateHandle().didChangeDependencies = true
		}
	}
}

func (el *SimpleInteriorElement[W]) GetChild() Element {
	return el.child
}

func (el *SimpleInteriorElement[W]) SetChild(child Element) {
	el.child = child
}

func (el *SimpleInteriorElement[W]) GetState() State[W] {
	return el.State
}

func (el *SimpleInteriorElement[W]) Build() Widget {
	if s := el.State; s != nil {
		return s.Build()
	} else if w, ok := el.widget.(WidgetBuilder); ok {
		return w.Build()
	} else {
		panic(fmt.Sprintf("widget %T needs to implement WidgetBuilder or StatefulWidget", el.widget))
	}
}

func (el *SimpleInteriorElement[W]) PerformRebuild() {
	if s := el.State; s != nil {
		h := el.GetStateHandle()
		if h.didChangeDependencies {
			s.Transition(StateTransition[W]{Kind: StateChangedDependencies})
			h.didChangeDependencies = false
		}
	}
	built := el.Build()
	el.SetChild(UpdateChild(el, el.GetChild(), built, el.Handle().slot))
	el.Handle().dirty = false
}

type BuildContext interface{}

type Widget interface {
	CreateElement() Element
}

type KeyedWidget interface {
	Widget

	GetKey() any
}

type StatelessWidget interface {
	Widget
	// XXX StatelessWidget and WidgetBuilder disagree about the signature.
	Build(ctx BuildContext) Widget
}

type StatefulWidget[W Widget] interface {
	Widget
	CreateState() State[W]
}

type ParentDataWidget interface {
	Widget
	ApplyParentData(obj render.Object)
}

// State is state.
type State[W Widget] interface {
	WidgetBuilder

	GetStateHandle() *StateHandle[W]
	Transition(t StateTransition[W])
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
	HasChildElement
	WidgetBuilder
}

func DidChangeDependencies(el Element) {
	MarkNeedsBuild(el)
	el.Transition(ElementTransition{Kind: ElementChangedDependencies})
}

func Update(el Element, newWidget Widget) {
	oldWidget := el.Handle().widget
	el.Handle().widget = newWidget
	el.Transition(ElementTransition{Kind: ElementUpdated, OldWidget: oldWidget})
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
	h.BuildOwner.scheduleBuildFor(el)
}

func AttachRenderObject(el Element, slot int) {
	type renderObjectAttacher interface {
		AttachRenderObject(slot int)
	}

	if el, ok := el.(renderObjectAttacher); ok {
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
	el.Handle().slot = int(math.MinInt)
	if el, ok := el.(RenderObjectDetacher); ok {
		el.AfterDetachRenderObject()
	}
}

type SlotUpdater interface {
	AfterUpdateSlot(oldSlot, newSlot int)
}

// UpdateSlot updates the element's slot. If the element implements SlotUpdater, AfterUpdateSlot is called
// afterwards with the old and new slots.
func UpdateSlot(el Element, newSlot int) {
	h := el.Handle()
	old := h.slot
	h.slot = newSlot
	if el, ok := el.(SlotUpdater); ok {
		el.AfterUpdateSlot(old, newSlot)
	}
}

func UpdateChild(el, child Element, newWidget Widget, newSlot int) Element {
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
		h.BuildOwner.scheduleBuildFor(el)
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

func Mount(el, parent Element, newSlot int) {
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
		h.BuildOwner = parent.Handle().BuildOwner
	}

	el.Handle().dirty = true
	el.Transition(ElementTransition{Kind: ElementMounted, Parent: parent, NewSlot: newSlot})
}

// Unmount unmounts the element.
func Unmount(el Element) {
	h := el.Handle()
	if keyer, ok := h.widget.(KeyedWidget); ok {
		key := keyer.GetKey()
		if key, ok := key.(*GlobalKey); ok {
			_ = key
			// owner!._unregisterGlobalKey(key, this); // XXX
		}
	}
	h.lifecycleState = ElementLifecycleDefunct

	el.Transition(ElementTransition{Kind: ElementUnmounted})
}

type ChildrenVisiter interface {
	VisitChildren(yield func(el Element) bool)
}

// VisitChildren visits an element's children, by using its VisitChildren or GetChild methods.
func VisitChildren(el Element, yield func(child Element) bool) {
	if el, ok := el.(ChildrenVisiter); ok {
		el.VisitChildren(yield)
	}
}

type ChildForgetter interface {
	ForgetChild(child Element)
}

// ForgetChild instructs an element to forget one of its children by calling ForgetChild if possible.
func ForgetChild(el Element, child Element) {
	if el, ok := el.(ChildForgetter); ok {
		el.ForgetChild(child)
	}
}

// XXX find a better name
type HasChildElement interface {
	Element
	VisitChildren(yield func(e Element) bool)
	// XXX figure out a better API
	Children() []Element
	SetChildren(children []Element)
	ForgottenChildren() map[Element]struct{}
}

type WidgetBuilder interface {
	Build() Widget
}

var _ RenderObjectElement = (*SimpleRenderObjectElement)(nil)

type SimpleRenderObjectElement struct {
	RenderObjectElementHandle
	children          []Element
	forgottenChildren map[Element]struct{}
}

// SetChildren implements HasChildRenderObjectElement.
func (el *SimpleRenderObjectElement) SetChildren(children []Element) {
	el.children = children
	clear(el.forgottenChildren)
}

// AttachRenderObject implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) AttachRenderObject(slot int) {
	RenderObjectElementAttachRenderObject(el, slot)
}

// InsertRenderObjectChild implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) InsertRenderObjectChild(child render.Object, slot int) {
	RenderObjectElementInsertRenderObjectChild(el, child, slot)
}

// MoveRenderObjectChild implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) MoveRenderObjectChild(child render.Object, newSlot int) {
	RenderObjectElementMoveRenderObjectChild(el, child, newSlot)
}

// RemoveRenderObjectChild implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) RemoveRenderObjectChild(child render.Object, slot int) {
	RenderObjectElementRemoveRenderObjectChild(el, child, slot)
}

// PerformRebuild implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) PerformRebuild() {
	RenderObjectElementPerformRebuild(el)
}

func (el *SimpleRenderObjectElement) VisitChildren(yield func(el Element) bool) {
	RenderObjectElementVisitChildren(el, yield)
}

// Children implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) Children() []Element {
	return el.children
}

// ForgottenChildren implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) ForgottenChildren() map[Element]struct{} {
	return el.forgottenChildren
}

func (el *SimpleRenderObjectElement) ForgetChild(child Element) {
	RenderObjectElementForgetChild(el, child)
}

// Transition implements MultiChildRenderObjectElement.
func (el *SimpleRenderObjectElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementMounted:
		RenderObjectElementAfterMount(el, t.Parent, t.NewSlot)
	case ElementUnmounted:
		RenderObjectElementAfterUnmount(el)
	case ElementUpdated:
		RenderObjectElementAfterUpdate(el, t.OldWidget.(RenderObjectWidget))
	}
}

// XXX rename this
type BuildOwner struct {
	dirtyElements               []Element
	inactiveElements            inactiveElements
	dirtyElementsNeedsResorting bool
	OnBuildScheduled            func()
	scheduledFlushDirtyElements bool
	PipelineOwner               *render.PipelineOwner
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

type StateHandle[W Widget] struct {
	Widget                W
	Element               Element
	didChangeDependencies bool
}

type ElementHandle struct {
	parent         Element
	slot           int
	lifecycleState int
	depth          int
	BuildOwner     *BuildOwner
	dirty          bool
	inDirtyList    bool
	widget         Widget
}

func (h *StateHandle[W]) GetStateHandle() *StateHandle[W] { return h }

func (el *ElementHandle) Handle() *ElementHandle { return el }
func (el *ElementHandle) Parent() Element        { return el.parent }
func (el *ElementHandle) Slot() any              { return el.slot }

type RenderObjectElementHandle struct {
	ElementHandle
	RenderObject                render.Object
	ancestorRenderObjectElement RenderObjectElement
}

func (el *RenderObjectElementHandle) RenderHandle() *RenderObjectElementHandle {
	return el
}

func (h *RenderObjectElementHandle) UpdateSlot(oldSlot, newSlot int) {
	if ancestor := h.ancestorRenderObjectElement; ancestor != nil {
		ancestor.MoveRenderObjectChild(h.RenderObject, h.slot)
	}
}

func (el *RenderObjectElementHandle) AfterDetachRenderObject() {
	if el.ancestorRenderObjectElement != nil {
		el.ancestorRenderObjectElement.RemoveRenderObjectChild(el.RenderObject, el.slot)
		el.ancestorRenderObjectElement = nil
	}
	el.slot = 0
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
	if !sameType(old, new) {
		return false
	}
	var key1, key2 any
	if old, ok := old.(KeyedWidget); ok {
		key1 = old.GetKey()
		if new, ok := new.(KeyedWidget); ok {
			key2 = new.GetKey()
		}
	}
	return key1 == key2
}

func deactivateChild(child Element) {
	child.Handle().parent = nil
	DetachRenderObject(child)
	child.Handle().BuildOwner.inactiveElements.add(child)
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

func InflateWidget(parent Element, widget Widget, slot int) Element {
	if widget, ok := widget.(KeyedWidget); ok {
		key := widget.GetKey()
		if key, ok := key.(GlobalKey); ok {
			newChild := RetakeInactiveElement(parent, key, widget)
			if newChild != nil {
				activateWithParent(newChild, parent, slot)
				updatedChild := UpdateChild(parent, newChild, widget, slot)
				return updatedChild
			}
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
	el.Handle().BuildOwner.inactiveElements.remove(element)
	return element
}

func activateWithParent(el, parent Element, newSlot int) {
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

func updateSlotForChild(el, child Element, newSlot int) {
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

func UpdateChildren(el HasChildElement, newWidgets []Widget, forgottenChildren map[Element]struct{}) []Element {
	oldChildren := el.Children()
	replaceWithNilIfForgotten := func(child Element) Element {
		if _, ok := forgottenChildren[child]; ok {
			return nil
		} else {
			return child
		}
	}

	// This attempts to diff the new child list (newWidgets) with
	// the old child list (oldChildren), and produce a new list of elements to
	// be the new list of child elements of this element. The called of this
	// method is expected to update this render object accordingly.

	// The cases it tries to optimize for are:
	//  - the old list is empty
	//  - the lists are identical
	//  - there is an insertion or removal of one or more widgets in
	//    only one place in the list
	// If a widget with a key is in both lists, it will be synced.
	// Widgets without keys might be synced but there is no guarantee.

	// The general approach is to sync the entire new list backwards, as follows:
	// 1. Walk the lists from the top, syncing nodes, until you no longer have
	//    matching nodes.
	// 2. Walk the lists from the bottom, without syncing nodes, until you no
	//    longer have matching nodes. We'll sync these nodes at the end. We
	//    don't sync them now because we want to sync all the nodes in order
	//    from beginning to end.
	// At this point we narrowed the old and new lists to the point
	// where the nodes no longer match.
	// 3. Walk the narrowed part of the old list to get the list of
	//    keys and sync null with non-keyed items.
	// 4. Walk the narrowed part of the new list forwards:
	//     * Sync non-keyed items with null
	//     * Sync keyed items with the source if it exists, else with null.
	// 5. Walk the bottom of the list again, syncing the nodes.
	// 6. Sync null with any items in the list of keys that are still
	//    mounted.

	newChildrenTop := 0
	oldChildrenTop := 0
	newChildrenBottom := len(newWidgets) - 1
	oldChildrenBottom := len(oldChildren) - 1

	newChildren := make([]Element, len(newWidgets))

	// Update the top of the list.
	for (oldChildrenTop <= oldChildrenBottom) && (newChildrenTop <= newChildrenBottom) {
		oldChild := replaceWithNilIfForgotten(oldChildren[oldChildrenTop])
		newWidget := newWidgets[newChildrenTop]
		if oldChild == nil || !canUpdate(oldChild.Handle().widget, newWidget) {
			break
		}
		newChild := UpdateChild(el, oldChild, newWidget, newChildrenTop)
		newChildren[newChildrenTop] = newChild
		newChildrenTop++
		oldChildrenTop++
	}

	// Scan the bottom of the list.
	for (oldChildrenTop <= oldChildrenBottom) && (newChildrenTop <= newChildrenBottom) {
		oldChild := replaceWithNilIfForgotten(oldChildren[oldChildrenBottom])
		newWidget := newWidgets[newChildrenBottom]
		if oldChild == nil || !canUpdate(oldChild.Handle().widget, newWidget) {
			break
		}
		oldChildrenBottom--
		newChildrenBottom--
	}

	Key := func(w Widget) any {
		if w, ok := w.(KeyedWidget); ok {
			return w.GetKey()
		} else {
			return nil
		}
	}

	// Scan the old children in the middle of the list.
	haveOldChildren := oldChildrenTop <= oldChildrenBottom
	var oldKeyedChildren map[any]Element
	if haveOldChildren {
		oldKeyedChildren = map[any]Element{}
		for oldChildrenTop <= oldChildrenBottom {
			oldChild := replaceWithNilIfForgotten(oldChildren[oldChildrenTop])
			if oldChild != nil {
				if Key(oldChild.Handle().widget) != nil {
					oldKeyedChildren[Key(oldChild.Handle().widget)] = oldChild
				} else {
					deactivateChild(oldChild)
				}
			}
			oldChildrenTop++
		}
	}

	// Update the middle of the list.
	for newChildrenTop <= newChildrenBottom {
		var oldChild Element
		newWidget := newWidgets[newChildrenTop]
		if haveOldChildren {
			key := Key(newWidget)
			if key != nil {
				oldChild = oldKeyedChildren[key]
				if oldChild != nil {
					if canUpdate(oldChild.Handle().widget, newWidget) {
						// we found a match!
						// remove it from oldKeyedChildren so we don't unsync it later
						delete(oldKeyedChildren, key)
					} else {
						// Not a match, let's pretend we didn't see it for now.
						oldChild = nil
					}
				}
			}
		}
		newChild := UpdateChild(el, oldChild, newWidget, newChildrenTop)
		newChildren[newChildrenTop] = newChild
		newChildrenTop++
	}

	// We've scanned the whole list.
	newChildrenBottom = len(newWidgets) - 1
	oldChildrenBottom = len(oldChildren) - 1

	// Update the bottom of the list.
	for (oldChildrenTop <= oldChildrenBottom) && (newChildrenTop <= newChildrenBottom) {
		oldChild := oldChildren[oldChildrenTop]
		newWidget := newWidgets[newChildrenTop]
		newChild := UpdateChild(el, oldChild, newWidget, newChildrenTop)
		newChildren[newChildrenTop] = newChild
		newChildrenTop++
		oldChildrenTop++
	}

	// Clean up any of the remaining middle nodes from the old list.
	for _, oldChild := range oldKeyedChildren {
		if _, ok := forgottenChildren[oldChild]; !ok {
			deactivateChild(oldChild)
		}
	}
	return newChildren
}

func ApplyParentData(pd ParentDataWidget, childrenOf Element) {
	var applyParentData func(child Element)
	applyParentData = func(child Element) {
		if rto, ok := child.(RenderObjectElement); ok {
			pd.ApplyParentData(rto.RenderHandle().RenderObject)
		} else {
			VisitChildren(child, func(child Element) bool {
				applyParentData(child)
				return true
			})
		}
	}
	applyParentData(childrenOf)
}

func GetWidgetChild(parent Widget) Widget {
	v := reflect.Indirect(reflect.ValueOf(parent))
	if f := v.FieldByName("Child"); f.IsValid() {
		if f.IsNil() {
			return nil
		} else {
			return f.Interface().(Widget)
		}
	} else if f := v.FieldByName("Children"); f.IsValid() {
		return f.Index(0).Interface().(Widget)
	} else {
		panic(fmt.Sprintf("%T does not have children", parent))
	}
}

func WidgetChildrenIter(parent Widget) func(yield func(i int, w Widget) bool) {
	v := reflect.Indirect(reflect.ValueOf(parent))
	if f := v.FieldByName("Children"); f.IsValid() {
		if f.Len() == 0 {
			return func(yield func(i int, w Widget) bool) {}
		}
		return func(yield func(i int, w Widget) bool) {
			n := f.Len()
			for i := range n {
				if !yield(i, f.Index(i).Interface().(Widget)) {
					break
				}
			}
		}
	} else if f := v.FieldByName("Child"); f.IsValid() {
		if f.IsNil() {
			return func(yield func(i int, w Widget) bool) {}
		}
		return func(yield func(i int, w Widget) bool) {
			yield(0, f.Interface().(Widget))
		}
	} else {
		panic(fmt.Sprintf("%T does not have children", parent))
	}
}

func WidgetNumChildren(parent Widget) int {
	v := reflect.Indirect(reflect.ValueOf(parent))
	if f := v.FieldByName("Child"); f.IsValid() {
		if f.IsNil() {
			return 0
		} else {
			return 1
		}
	} else if f := v.FieldByName("Children"); f.IsValid() {
		return f.Len()
	} else {
		panic(fmt.Sprintf("%T does not have children", parent))
	}
}

func WidgetChildren(parent Widget) []Widget {
	v := reflect.Indirect(reflect.ValueOf(parent))
	if f := v.FieldByName("Children"); f.IsValid() {
		return f.Interface().([]Widget)
	} else if f := v.FieldByName("Child"); f.IsValid() {
		if f.IsNil() {
			return nil
		} else {
			return []Widget{f.Interface().(Widget)}
		}
	} else {
		panic(fmt.Sprintf("%T does not have children", parent))
	}
}
