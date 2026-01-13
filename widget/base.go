// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

// Package widget implements the widget tree. For actual widgets, see
// [honnef.co/go/gutter/widget/widgets].
package widget

import (
	"fmt"
	"iter"
	"maps"
	"math"
	"reflect"
	"slices"
	"unsafe"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/wsi"
)

type elementTransitionKind uint8

const (
	elementMounted elementTransitionKind = iota
	elementChangedDependencies
	elementUpdated
	elementDeactivating
	elementActivated
	elementUnmounted
)

//go:generate stringer -type=StateTransitionKind
type StateTransitionKind uint8

const (
	// StateInitializing gets emitted exactly once when a stateful widget's
	// state is first created.
	StateInitializing StateTransitionKind = iota
	// StateUpdatedWidget gets emitted when the widget associated with its state
	// changes. Two widgets are considered different when their addresses
	// differ, even if their values are identical.
	StateUpdatedWidget
	// StateChangedDependencies gets emitted when a widget that the state
	// depends on has changed.
	StateChangedDependencies
	// StateDeactivating gets emitted when a widget and its state are removed from
	// the widget tree. This might be followed by either StateActivating or
	// StateDisposing.
	StateDeactivating
	// StateActivating gets emitted when a widget and its state that have
	// previously been removed are added back to the widget tree.
	StateActivating
	// StateDisposing gets emitted at most once per instance of state at the end
	// of its lifecycle, when the widget and its associated state are about to
	// be permanently removed from the widget tree.
	StateDisposing
)

type renderObjectElement interface {
	parentElement

	renderHandle() *renderObjectElementHandle

	insertRenderObjectChild(child render.Object, slot int)
	removeRenderObjectChild(child render.Object, slot int)
	moveRenderObjectChild(child render.Object, newSlot int)

	attachRenderObject(slot int)
}

type elementTransition struct {
	kind elementTransitionKind

	// The old widget for Kind == ElementUpdated
	oldWidget Widget

	// The parent and slot for Kind == ElementMounted
	parent  Element
	newSlot int
}

type StateTransition[W Widget] struct {
	Kind StateTransitionKind

	// The old widget for Kind == StateUpdatedWidget
	OldWidget W
}

// TODO support "Notification"

func updateAncestors(el Element) {
	debug.Assert(el.handle().lifecycleState == elementLifecycleActive)
	var incomingWidgets map[reflect.Type]Element
	h := el.handle()
	if p := h.parent; p != nil {
		incomingWidgets = maps.Clone(p.handle().ancestorElements)
		if incomingWidgets == nil {
			incomingWidgets = map[reflect.Type]Element{}
		}
	} else {
		incomingWidgets = map[reflect.Type]Element{}
	}
	incomingWidgets[reflect.TypeOf(h.widget)] = el
	h.ancestorElements = incomingWidgets
}

func NewInteriorElement[W Widget](w W) Element {
	se := &simpleInteriorElement[W]{}
	se.elementHandle.widget = w
	if w2, ok := any(w).(StatefulWidget[W]); ok {
		se.State = w2.CreateState()
		sh := se.State.GetStateHandle()
		sh.Widget = w
		sh.Element = se
	}
	return se
}

type simpleInteriorElement[W Widget] struct {
	elementHandle
	State[W]
	singleChildElement
}

func (el *simpleInteriorElement[W]) transition(t elementTransition) {
	switch t.kind {
	case elementMounted:
		if s := el.State; s != nil {
			s.Transition(StateTransition[W]{Kind: StateInitializing})
			s.Transition(StateTransition[W]{Kind: StateChangedDependencies})
		}
		rebuild(el)
	case elementActivated:
		if s := el.State; s != nil {
			s.Transition(StateTransition[W]{Kind: StateActivating})
		}
		MarkNeedsBuild(el)
	case elementUnmounted:
		if s := el.State; s != nil {
			h := s.GetStateHandle()
			s.Transition(StateTransition[W]{Kind: StateDisposing})
			h.Element = nil
		}
	case elementUpdated:
		if s := el.State; s != nil {
			h := el.GetStateHandle()
			oldWidget := h.Widget
			h.Widget = el.handle().widget.(W)
			s.Transition(StateTransition[W]{Kind: StateUpdatedWidget, OldWidget: oldWidget})
		}
		forceRebuild(el)
	case elementDeactivating:
		if s := el.State; s != nil {
			s.Transition(StateTransition[W]{Kind: StateDeactivating})
		}
	case elementChangedDependencies:
		if s := el.State; s != nil {
			el.GetStateHandle().didChangeDependencies = true
		}
	}
}

func (el *simpleInteriorElement[W]) GetState() State[W] {
	// XXX can we delete this?
	return el.State
}

func (el *simpleInteriorElement[W]) Build() Widget {
	if s := el.State; s != nil {
		return s.Build(el)
	} else if w, ok := el.widget.(WidgetBuilder); ok {
		return w.Build(el)
	} else {
		panic(fmt.Sprintf("widget %T needs to implement widget.WidgetBuilder or %s", el.widget, reflect.TypeFor[StatefulWidget[W]]()))
	}
}

func (el *simpleInteriorElement[W]) performRebuild() {
	if s := el.State; s != nil {
		h := el.GetStateHandle()
		if h.didChangeDependencies {
			s.Transition(StateTransition[W]{Kind: StateChangedDependencies})
			h.didChangeDependencies = false
		}
	}
	built := el.Build()
	el.SetChild(updateChild(el, el.Child(), built, el.handle().slot))
	el.handle().dirty = false
}

// Ancestor returns the earliest ancestor of bc that has type W and establishes
// a dependency on it.
func Ancestor[W Widget](bc BuildContext) W {
	el := bc.(Element)
	h := el.handle()
	if ancestor := h.ancestorElements[reflect.TypeFor[W]()]; ancestor != nil {
		if h.dependencies == nil {
			h.dependencies = make(map[Element]struct{})
		}
		h.dependencies[ancestor] = struct{}{}
		ah := ancestor.handle()
		if ah.dependents == nil {
			ah.dependents = make(map[Element]struct{})
		}
		ah.dependents[el] = struct{}{}
		return ah.widget.(W)
	}
	h.hadUnsatisfiedDependencies = true
	return *new(W)
}

type BuildContext interface {
}

// A Widget is a declarative, immutable description of part of a UI. Widgets
// form a directed, acyclic graph.
//
// Gutter uses widgets to build and maintain an [Element] tree, which is a
// concrete instantiation of the UI, linking the declarative, immutable
// description to mutable state.
//
// Gutter knows several specialized kinds of widgets:
//   - [StatelessWidget]
//   - [StatefulWidget]
//   - [RenderObjectWidget]
//   - [KeyedWidget]
//   - [ParentDataWidget]
type Widget interface {
}

type KeyedWidget interface {
	Widget

	GetKey() any
}

type StatelessWidget interface {
	Widget
	Build(ctx BuildContext) Widget
}

type StatefulWidget[W Widget] interface {
	Widget
	// CreateElement returns a new Element that represents this widget in the
	// element tree.
	//
	// You should use one of the following functions to implement CreateElement:
	//
	//   - [NewInteriorElement] for most [StatelessWidget]s and [StatefulWidget]s
	//   - [NewRenderObjectElement] for any [RenderObjectWidget].
	//
	// See the documentation on [Element] for more information about the element
	// tree.
	CreateElement() Element
	CreateState() State[W]
}

type ParentDataWidget interface {
	Widget
	ApplyParentData(obj render.Object)
}

// State is state.
type State[W Widget] interface {
	// Build builds a widget subtree that represents this widget. High-level
	// widgets are composed of lower-level widgets. This process happens
	// recursively until we're left with basic widgets, usually
	// [RenderObjectWidget]s.
	Build(ctx BuildContext) Widget

	// GetStateHandle returns the state's handle, which is metadata about the
	// state that is maintained by the Gutter runtime.
	//
	// To implement this method, embed StateHandle[W].
	GetStateHandle() *StateHandle[W]

	// Transition notifies the state of a state transition as a result of the
	// widget tree changing. See the documentation of [StateTransitionKind] for
	// a description of the possible transitions.
	Transition(t StateTransition[W])
}

// A RenderObjectWidget is a widget that maps to a [render.Object]. Widgets of
// this kind are responsible for putting actual pixels on the screen and form
// the leaf nodes of the widget tree.
type RenderObjectWidget interface {
	Widget

	// CreateRenderObject creates a new render object, configured as described
	// by the widget. All calls must return the same concrete type of render
	// object.
	CreateRenderObject(ctx BuildContext) render.Object

	// UpdateRenderObject updates an existing render object to match the widget.
	// The concrete type of obj will always be the same as that returned by
	// CreateRenderObject.
	UpdateRenderObject(ctx BuildContext, obj render.Object)
}

type Element interface {
	handle() *elementHandle
	transition(t elementTransition)
	performRebuild()
	children() iter.Seq[Element]
}

type interiorElement interface {
	parentElement
	build() Widget
}

func didChangeDependencies(el Element) {
	MarkNeedsBuild(el)
	el.transition(elementTransition{kind: elementChangedDependencies})
}

func update(el Element, newWidget Widget) {
	h := el.handle()
	oldWidget := h.widget
	h.widget = newWidget
	if pd, ok := h.widget.(ParentDataWidget); ok {
		applyParentData(pd, el)
	}
	for dependent := range h.dependents {
		// OPT(dh): introduce UpdateShouldNotify
		didChangeDependencies(dependent)
	}
	el.transition(elementTransition{kind: elementUpdated, oldWidget: oldWidget})
}

func renderObjectAttachingChild(el Element) Element {
	if _, ok := el.(renderObjectElement); ok {
		return nil
	}
	var out Element
	for child := range el.children() {
		debug.Assert(out == nil)
		out = child
	}
	return out
}

func MarkNeedsBuild(el Element) {
	h := el.handle()
	if h.lifecycleState != elementLifecycleActive {
		return
	}
	if h.dirty {
		return
	}
	h.dirty = true
	h.BuildOwner.scheduleBuildFor(el)
}

func attachRenderObject(el Element, slot int) {
	type renderObjectAttacher interface {
		attachRenderObject(slot int)
	}

	if el, ok := el.(renderObjectAttacher); ok {
		el.attachRenderObject(slot)
		return
	}
	for child := range el.children() {
		attachRenderObject(child, slot)
	}
	el.handle().slot = slot
}

type renderObjectDetacher interface {
	performDetachRenderObject()
}

// detachRenderObject recursively instructs the children of the element to detach their render object.
// Elements that implement RenderObjectDetacher have their AfterDetachRenderObject method called instead.
func detachRenderObject(el Element) {
	if el, ok := el.(renderObjectDetacher); ok {
		el.performDetachRenderObject()
		return
	}
	for child := range el.children() {
		detachRenderObject(child)
	}
	el.handle().slot = int(math.MinInt)
}

type slotUpdater interface {
	afterUpdateSlot(oldSlot, newSlot int)
}

// updateSlot updates the element's slot. If the element implements SlotUpdater, afterUpdateSlot is called
// afterwards with the old and new slots.
func updateSlot(el Element, newSlot int) {
	h := el.handle()
	old := h.slot
	h.slot = newSlot
	if el, ok := el.(slotUpdater); ok {
		el.afterUpdateSlot(old, newSlot)
	}
}

func updateChild(el, child Element, newWidget Widget, newSlot int) Element {
	if newWidget == nil {
		if child != nil {
			deactivateChild(child)
		}
		return nil
	}

	var newChild Element
	if child != nil {
		if child.handle().widget == newWidget {
			if child.handle().slot != newSlot {
				updateSlotForChild(el, child, newSlot)
			}
			newChild = child
		} else if canUpdate(child.handle().widget, newWidget) {
			if child.handle().slot != newSlot {
				updateSlotForChild(el, child, newSlot)
			}
			update(child, newWidget)
			newChild = child
		} else {
			deactivateChild(child)
			newChild = inflateWidget(el, newWidget, newSlot)
		}
	} else {
		newChild = inflateWidget(el, newWidget, newSlot)
	}

	return newChild
}

// activate activates the element.
func activate(el Element) {
	debug.Assert(el.handle().lifecycleState == elementLifecycleInactive)
	hadDependencies := len(el.handle().dependencies) != 0 || el.handle().hadUnsatisfiedDependencies

	h := el.handle()
	h.lifecycleState = elementLifecycleActive
	// We unregistered our dependencies in deactivate, but never cleared the list.
	// Since we're going to be reused, let's clear our list now.
	clear(el.handle().dependencies)
	el.handle().hadUnsatisfiedDependencies = false
	updateAncestors(el)
	// el.attachNotificationTree()
	if h.dirty {
		h.BuildOwner.scheduleBuildFor(el)
	}
	if hadDependencies {
		didChangeDependencies(el)
	}

	el.transition(elementTransition{kind: elementActivated})
}

func deactivate(el Element) {
	el.transition(elementTransition{kind: elementDeactivating})

	for dependency := range el.handle().dependencies {
		delete(dependency.handle().dependents, el)
		// For expediency, we don't actually clear the list here, even though it's
		// no longer representative of what we are registered with. If we never
		// get re-used, it doesn't matter. If we do, then we'll clear the list in
		// activate(). The benefit of this is that it allows Element's activate()
		// implementation to decide whether to rebuild based on whether we had
		// dependencies here.
	}
	el.handle().ancestorElements = nil

	el.handle().lifecycleState = elementLifecycleInactive
}

func mount(el, parent Element, newSlot int) {
	h := el.handle()
	h.parent = parent
	h.slot = newSlot
	h.lifecycleState = elementLifecycleActive
	if parent != nil {
		h.depth = parent.handle().depth + 1
	} else {
		h.depth = 1
	}
	if parent != nil {
		// Only assign ownership if the parent is non-null. If parent is null
		// (the root node), the owner should have already been assigned.
		h.BuildOwner = parent.handle().BuildOwner
	}

	if widget, ok := h.widget.(KeyedWidget); ok {
		if key, ok := widget.GetKey().(GlobalKey); ok {
			h.BuildOwner.registerGlobalKey(key, el)
		}
	}

	updateAncestors(el)

	// XXX attachNotificationTree

	el.handle().dirty = true
	el.transition(elementTransition{kind: elementMounted, parent: parent, newSlot: newSlot})
}

// unmount unmounts the element.
func unmount(el Element) {
	h := el.handle()
	if keyer, ok := h.widget.(KeyedWidget); ok {
		if key, ok := keyer.GetKey().(GlobalKey); ok {
			h.BuildOwner.unregisterGlobalKey(key, el)
		}
	}
	h.lifecycleState = elementLifecycleDefunct

	el.transition(elementTransition{kind: elementUnmounted})
}

type childForgetter interface {
	forgetChild(child Element)
}

// forgetChild instructs an element to forget one of its children by calling forgetChild if possible.
func forgetChild(el Element, child Element) {
	if el, ok := el.(childForgetter); ok {
		el.forgetChild(child)
	}
}

type parentElement interface {
	Element
	// XXX figure out a better API
	getChildren() []Element
	setChildren(children []Element)
	forgottenChildren() map[Element]struct{}
}

type WidgetBuilder interface {
	Build(ctx BuildContext) Widget
}

var _ renderObjectElement = (*simpleRenderObjectElement)(nil)

type simpleRenderObjectElement struct {
	renderObjectElementHandle
	manyChildElements
}

// attachRenderObject implements renderObjectElement.
func (el *simpleRenderObjectElement) attachRenderObject(slot int) {
	renderObjectElementAttachRenderObject(el, slot)
}

// InsertRenderObjectChild implements renderObjectElement.
func (el *simpleRenderObjectElement) insertRenderObjectChild(child render.Object, slot int) {
	renderObjectElementInsertRenderObjectChild(el, child, slot)
}

// moveRenderObjectChild implements renderObjectElement.
func (el *simpleRenderObjectElement) moveRenderObjectChild(child render.Object, newSlot int) {
	if newSlot >= 0 {
		newSlot--
	}
	render.MoveChild(renderObjectElement(el).renderHandle().RenderObject.(render.ObjectWithChildren), child, newSlot)
}

// removeRenderObjectChild implements renderObjectElement.
func (el *simpleRenderObjectElement) removeRenderObjectChild(child render.Object, slot int) {
	renderObjectElementRemoveRenderObjectChild(el, child, slot)
}

// performRebuild implements renderObjectElement.
func (el *simpleRenderObjectElement) performRebuild() {
	h := renderObjectElement(el).renderHandle()
	h.widget.(RenderObjectWidget).UpdateRenderObject(renderObjectElement(el), h.RenderObject)
	renderObjectElement(el).handle().dirty = false
}

// Transition implements renderObjectElement.
func (el *simpleRenderObjectElement) transition(t elementTransition) {
	switch t.kind {
	case elementMounted:
		renderObjectElementAfterMount(el, t.parent, t.newSlot)
	case elementUnmounted:
		h := renderObjectElement(el).renderHandle()
		oldWidget := h.widget.(RenderObjectWidget)
		if n, ok := oldWidget.(renderObjectUnmountNotifyee); ok {
			n.DidUnmountRenderObject(h.RenderObject)
		}
		render.Dispose(h.RenderObject)
		h.RenderObject = nil
	case elementUpdated:
		renderObjectElementAfterUpdate(el, t.oldWidget.(RenderObjectWidget))
	}
}

// XXX rename this
type BuildOwner struct {
	dirtyElements               []Element
	inactiveElements            inactiveElements
	dirtyElementsNeedsResorting bool
	OnBuildScheduled            func()
	scheduledFlushDirtyElements bool
	Renderer                    *render.Renderer
	EmitEvent                   func(ev wsi.Event)
	globals                     map[GlobalKey]Element
	inDrawFrame                 bool
}

// CreateTicker implements animation.TickerProvider.
func (o *BuildOwner) CreateTicker(cb animation.TickerCallback) animation.Ticker {
	// TODO(dh): eventually we'll want a widget tree-aware ticker provider that
	// supports something akin to Flutter's TickerMode.
	tp := &animation.PlainTickerProvider{
		FrameCallbacker: o.Renderer,
	}
	return tp.CreateTicker(cb)
}

func NewBuildOwner() *BuildOwner {
	return &BuildOwner{
		globals: make(map[GlobalKey]Element),
	}
}

func (o *BuildOwner) registerGlobalKey(key GlobalKey, el Element) {
	o.globals[key] = el
}

func (o *BuildOwner) unregisterGlobalKey(key GlobalKey, el Element) {
	if o.globals[key] == el {
		// We have to check the element because of this sequence of events:
		//
		// 1. add key with elA
		// 2. mark elA as inactive
		// 3. add key with elB
		// 4. unmount inactive elements
		//
		// That is, we mount the new element before we unmount the old one, which causes the key to be
		// overwritten.
		delete(o.globals, key)
	} else {
		// If the types were identical, then we should've reused the element instead of replacing it with a
		// new one.
		debug.Assert(!sameType(el, o.globals[key]))
	}
}

func (o *BuildOwner) scheduleBuildFor(el Element) {
	if el.handle().inDirtyList {
		o.dirtyElementsNeedsResorting = true
		return
	}
	if !o.inDrawFrame && !o.scheduledFlushDirtyElements && o.OnBuildScheduled != nil {
		o.scheduledFlushDirtyElements = true
		o.OnBuildScheduled()
	}
	o.dirtyElements = append(o.dirtyElements, el)
	el.handle().inDirtyList = true
}

func (o *BuildOwner) BuildScope(callback func()) {
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
			for index > 0 && o.dirtyElements[index-1].handle().dirty {
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
		element.handle().inDirtyList = false
	}
	clear(o.dirtyElements)
	o.dirtyElements = o.dirtyElements[:0]
	o.scheduledFlushDirtyElements = false
	o.dirtyElementsNeedsResorting = false
}

func (o *BuildOwner) finalizeTree() {
	o.inactiveElements.unmountAll()
}

//go:generate stringer -type=Lifecycle --trimprefix=ElementLifecycle
type lifecycle uint8

const (
	elementLifecycleIdle lifecycle = iota
	elementLifecycleActive
	elementLifecycleInactive
	elementLifecycleDefunct
)

type StateHandle[W Widget] struct {
	Widget                W
	Element               Element
	didChangeDependencies bool
}

type elementHandle struct {
	parent         Element
	slot           int
	lifecycleState lifecycle
	depth          int
	BuildOwner     *BuildOwner
	dirty          bool
	inDirtyList    bool
	widget         Widget
	// OPT(dh): use a persistent data structure for ancestorElements
	ancestorElements           map[reflect.Type]Element
	dependencies               map[Element]struct{}
	dependents                 map[Element]struct{}
	hadUnsatisfiedDependencies bool
}

func (h *StateHandle[W]) GetStateHandle() *StateHandle[W] { return h }

func (h *StateHandle[W]) BuildOwner() *BuildOwner {
	return h.Element.handle().BuildOwner
}

func (el *elementHandle) handle() *elementHandle { return el }
func (el *elementHandle) Parent() Element        { return el.parent }
func (el *elementHandle) Slot() any              { return el.slot }

type renderObjectElementHandle struct {
	elementHandle
	RenderObject                render.Object
	ancestorRenderObjectElement renderObjectElement
}

func (el *renderObjectElementHandle) renderHandle() *renderObjectElementHandle {
	return el
}

func (el *renderObjectElementHandle) afterUpdateSlot(oldSlot, newSlot int) {
	if ancestor := el.ancestorRenderObjectElement; ancestor != nil {
		ancestor.moveRenderObjectChild(el.RenderObject, el.slot)
	}
}

func (el *renderObjectElementHandle) PerformDetachRenderObject() {
	if el.ancestorRenderObjectElement != nil {
		el.ancestorRenderObjectElement.removeRenderObjectChild(el.RenderObject, el.slot)
		el.ancestorRenderObjectElement = nil
	}
	el.slot = -1
}

type renderObjectUnmountNotifyee interface {
	DidUnmountRenderObject(obj render.Object)
}

func findAncestorRenderObjectElement(el renderObjectElement) renderObjectElement {
	ancestor := el.handle().Parent()
	for ancestor != nil {
		if _, ok := ancestor.(renderObjectElement); ok {
			break
		}
		ancestor = ancestor.handle().Parent()
	}
	if ancestor == nil {
		return nil
	}
	return ancestor.(renderObjectElement)
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
	child.handle().parent = nil
	detachRenderObject(child)
	child.handle().BuildOwner.inactiveElements.add(child)
}

type inactiveElements struct {
	elements map[Element]struct{}
	locked   bool
}

func (els *inactiveElements) unmount(el Element) {
	for child := range el.children() {
		els.unmount(child)
	}
	unmount(el)
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
		ah := a.handle()
		bh := b.handle()
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
	deactivate(el)
	for child := range el.children() {
		els.deactivateRecursively(child)
	}
}

func (els *inactiveElements) add(el Element) {
	debug.Assert(!els.locked)
	_, ok := els.elements[el]
	debug.Assert(!ok)
	debug.Assert(el.handle().parent == nil)
	if el.handle().lifecycleState == elementLifecycleActive {
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

func inflateWidget(parent Element, widget Widget, newSlot int) Element {
	if widget, ok := widget.(KeyedWidget); ok {
		if key, ok := widget.GetKey().(GlobalKey); ok {
			newChild := retakeInactiveElement(parent, key, widget)
			if newChild != nil {
				activateWithParent(newChild, parent, newSlot)
				updatedChild := updateChild(parent, newChild, widget, newSlot)
				debug.Assert(newChild == updatedChild)
				return updatedChild
			}
		}
	}

	var newChild Element
	switch widget := widget.(type) {
	case interface{ CreateElement() Element }:
		newChild = widget.CreateElement()
	case RenderObjectWidget:
		newChild = NewRenderObjectElement(widget)
	default:
		newChild = NewInteriorElement(widget)
	}

	mount(newChild, parent, newSlot)
	return newChild
}

func retakeInactiveElement(el Element, key GlobalKey, newWidget Widget) Element {
	// The "inactivity" of the element being retaken here may be forward-looking: if
	// we are taking an element with a GlobalKey from an element that currently has
	// it as a child, then we know that element will soon no longer have that
	// element as a child. The only way that assumption could be false is if the
	// global key is being duplicated, and we'll try to track that using the
	// _debugTrackElementThatWillNeedToBeRebuiltDueToGlobalKeyShenanigans call below.

	element := el.handle().BuildOwner.globals[key]
	if element == nil {
		return nil
	}
	if !canUpdate(element.handle().widget, newWidget) {
		return nil
	}
	parent := element.handle().Parent()
	if parent != nil {
		forgetChild(parent, element)
		deactivateChild(element)
	}
	el.handle().BuildOwner.inactiveElements.remove(element)
	return element
}

func activateWithParent(el, parent Element, newSlot int) {
	debug.Assert(el.handle().lifecycleState == elementLifecycleInactive)
	el.handle().parent = parent
	updateDepth(el, parent.handle().depth)
	activateRecursively(el)
	attachRenderObject(el, newSlot)
	debug.Assert(el.handle().lifecycleState == elementLifecycleActive)
}

func updateDepth(el Element, parentDepth int) {
	expectedDepth := parentDepth + 1
	if el.handle().depth < expectedDepth {
		el.handle().depth = expectedDepth
		for child := range el.children() {
			updateDepth(child, expectedDepth)
		}
	}
}

func activateRecursively(el Element) {
	debug.Assert(el.handle().lifecycleState == elementLifecycleInactive)
	activate(el)
	debug.Assert(el.handle().lifecycleState == elementLifecycleActive)
	for child := range el.children() {
		activateRecursively(child)
	}
}

func updateSlotForChild(el, child Element, newSlot int) {
	for child != nil {
		updateSlot(child, newSlot)
		child = renderObjectAttachingChild(child)
	}
}

func rebuild(el Element) {
	if el.handle().lifecycleState != elementLifecycleActive || !el.handle().dirty {
		return
	}
	el.performRebuild()
	el.handle().dirty = false
}

func forceRebuild(el Element) {
	if el.handle().lifecycleState != elementLifecycleActive {
		return
	}
	el.performRebuild()
	el.handle().dirty = false
}

func updateChildren(el parentElement, newWidgets []Widget) []Element {
	oldChildren := el.getChildren()
	replaceWithNilIfForgotten := func(child Element) Element {
		if _, ok := el.forgottenChildren()[child]; ok {
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

	Key := func(w Widget) any {
		if w, ok := w.(KeyedWidget); ok {
			return w.GetKey()
		} else {
			return nil
		}
	}

	// Update the top of the list.
	for (oldChildrenTop <= oldChildrenBottom) && (newChildrenTop <= newChildrenBottom) {
		oldChild := replaceWithNilIfForgotten(oldChildren[oldChildrenTop])
		newWidget := newWidgets[newChildrenTop]
		if oldChild == nil || !canUpdate(oldChild.handle().widget, newWidget) {
			break
		}
		newChild := updateChild(el, oldChild, newWidget, newChildrenTop)
		newChildren[newChildrenTop] = newChild
		newChildrenTop++
		oldChildrenTop++
	}

	// Scan the bottom of the list.
	for (oldChildrenTop <= oldChildrenBottom) && (newChildrenTop <= newChildrenBottom) {
		oldChild := replaceWithNilIfForgotten(oldChildren[oldChildrenBottom])
		newWidget := newWidgets[newChildrenBottom]
		if oldChild == nil || !canUpdate(oldChild.handle().widget, newWidget) {
			break
		}
		newChildrenBottom--
		oldChildrenBottom--
	}

	// Scan the old children in the middle of the list.
	haveOldChildren := oldChildrenTop <= oldChildrenBottom
	var oldKeyedChildren map[any]Element
	if haveOldChildren {
		oldKeyedChildren = map[any]Element{}
		for oldChildrenTop <= oldChildrenBottom {
			oldChild := replaceWithNilIfForgotten(oldChildren[oldChildrenTop])
			if oldChild != nil {
				if Key(oldChild.handle().widget) != nil {
					oldKeyedChildren[Key(oldChild.handle().widget)] = oldChild
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
					if canUpdate(oldChild.handle().widget, newWidget) {
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
		newChild := updateChild(el, oldChild, newWidget, newChildrenTop)
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
		newChild := updateChild(el, oldChild, newWidget, newChildrenTop)
		newChildren[newChildrenTop] = newChild
		newChildrenTop++
		oldChildrenTop++
	}

	// Clean up any of the remaining middle nodes from the old list.
	for _, oldChild := range oldKeyedChildren {
		if _, ok := el.forgottenChildren()[oldChild]; !ok {
			deactivateChild(oldChild)
		}
	}
	return newChildren
}

func applyParentData(pd ParentDataWidget, childrenOf Element) {
	var applyParentData func(child Element)
	applyParentData = func(child Element) {
		if rto, ok := child.(renderObjectElement); ok {
			pd.ApplyParentData(rto.renderHandle().RenderObject)
		} else {
			for child2 := range child.children() {
				applyParentData(child2)
			}
		}
	}
	applyParentData(childrenOf)
}

func widgetChildrenIter(parent Widget) iter.Seq2[int, Widget] {
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
		return func(yield func(i int, w Widget) bool) {}
	}
}

func widgetChildren(parent Widget) []Widget {
	// XXX at some point we'll have widgets that have named children and just
	// looking for these two fields won't cut it.

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
		return nil
	}
}

type singleChildElement struct {
	child [1]Element
}

func (s *singleChildElement) getChildren() []Element {
	if s.child[0] == nil {
		return nil
	} else {
		return s.child[:]
	}
}

func (s *singleChildElement) forgottenChildren() map[Element]struct{} {
	return nil
}

func (s *singleChildElement) setChildren(children []Element) {
	debug.Assert(len(children) < 2)
	if len(children) == 0 {
		s.child[0] = nil
	} else {
		s.child[0] = children[0]
	}
}

func (s *singleChildElement) children() iter.Seq[Element] {
	return func(yield func(Element) bool) {
		if s.child[0] == nil {
			return
		}
		yield(s.child[0])
	}
}

func (s *singleChildElement) Child() Element {
	return s.child[0]
}

func (s *singleChildElement) SetChild(child Element) {
	s.child[0] = child
}

func (s *singleChildElement) forgetChild(child Element) {
	debug.Assert(s.child[0] == child)
	s.child[0] = nil
}

type manyChildElements struct {
	children_          []Element
	forgottenChildren_ map[Element]struct{}
}

func (m *manyChildElements) setChildren(children []Element) {
	m.children_ = children
	clear(m.forgottenChildren_)
}

func (m *manyChildElements) children() iter.Seq[Element] {
	return func(yield func(Element) bool) {
		forgotten := m.forgottenChildren_
		for _, child := range m.children_ {
			if _, ok := forgotten[child]; !ok {
				if !yield(child) {
					break
				}
			}
		}
	}
}

func (m *manyChildElements) getChildren() []Element {
	return m.children_
}

func (m *manyChildElements) forgottenChildren() map[Element]struct{} {
	return m.forgottenChildren_
}

func (m *manyChildElements) forgetChild(child Element) {
	if m.forgottenChildren_ == nil {
		m.forgottenChildren_ = make(map[Element]struct{})
	}
	m.forgottenChildren_[child] = struct{}{}
}

func NewRenderObjectElement(w RenderObjectWidget) Element {
	el := &simpleRenderObjectElement{}
	el.widget = w
	return el
}

var _ StatelessWidget = (*MediaQuery)(nil)

type MediaQuery struct {
	Data  MediaQueryData
	Child Widget
}

// Build implements StatelessWidget.
func (m *MediaQuery) Build(ctx BuildContext) Widget {
	return m.Child
}

type MediaQueryData struct {
	Scale float64
	Size  curve.Size
}
