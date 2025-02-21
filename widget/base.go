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
	"math"
	"reflect"
	"slices"
	"unsafe"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/mem"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/wsi"
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

//go:generate stringer -type=StateTransitionKind
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
	ParentElement

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

// TODO support "Notification"

func NewProxyElement[W Widget](w W) InteriorElement {
	el := &ProxyElement{}
	el.widget = w
	return el
}

type ProxyElement struct {
	ElementHandle
	SingleChildElement
}

// Build implements InteriorElement.
func (el *ProxyElement) Build() Widget {
	w := GetWidgetChild(el.widget)
	// TODO(dh): emit a more useful error than a generic assertion failure
	debug.Assert(w != nil)
	return w
}

// PerformRebuild implements InteriorElement.
func (el *ProxyElement) PerformRebuild() {
	built := el.Build()
	el.SetChild(UpdateChild(el, el.Child(), built, el.Handle().slot))
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
		// notifyClients(el, t.OldWidget)
	}
}

func NewInheritedElement[W Widget](w W) InheritedElement {
	se := &SimpleInheritedElement{}
	se.ElementHandle.widget = w
	// XXX do we care about StatefulWidget here, analogous to NewInteriorElement?
	return se
}

type InheritedElement interface {
	InteriorElement
	UpdateInheritance()
}

type SimpleInheritedElement struct {
	ProxyElement
}

func updateInheritance(el Element) {
	debug.Assert(el.Handle().lifecycleState == ElementLifecycleActive)
	if el, ok := el.(InheritedElement); ok {
		el.UpdateInheritance()
		return
	}
	h := el.Handle()
	if p := h.parent; p != nil {
		h.inheritedElements = p.Handle().inheritedElements
	} else {
		h.inheritedElements = nil
	}
}

func (el *SimpleInheritedElement) UpdateInheritance() {
	var incomingWidgets map[reflect.Type]InheritedElement
	h := el.Handle()
	if p := h.parent; p != nil {
		incomingWidgets = mem.CopyMap(p.Handle().inheritedElements)
	} else {
		incomingWidgets = map[reflect.Type]InheritedElement{}
	}
	incomingWidgets[reflect.TypeOf(h.widget)] = el
	h.inheritedElements = incomingWidgets
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
	SingleChildElement
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

func (el *SimpleInteriorElement[W]) GetState() State[W] {
	// XXX can we delete this?
	return el.State
}

func (el *SimpleInteriorElement[W]) Build() Widget {
	if s := el.State; s != nil {
		return s.Build(el)
	} else if w, ok := el.widget.(WidgetBuilder); ok {
		return w.Build(el)
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
	el.SetChild(UpdateChild(el, el.Child(), built, el.Handle().slot))
	el.Handle().dirty = false
}

func DependOnWidgetOfExactType[W Widget](bc BuildContext) W {
	el := bc.(Element)
	h := el.Handle()
	if ancestor := h.inheritedElements[reflect.TypeOf(*new(W))]; ancestor != nil {
		if h.dependencies == nil {
			h.dependencies = make(map[InheritedElement]struct{})
		}
		h.dependencies[ancestor] = struct{}{}
		ah := ancestor.Handle()
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
	// CreateElement returns a new Element that represents this widget in the
	// element tree.
	//
	// You should use one of the following functions to implement CreateElement:
	//
	//   - [NewInteriorElement] for most [StatelessWidget]s and [StatefulWidget]s
	//   - [NewRenderObjectElement] for any [RenderObjectWidget].
	//   - [NewProxyElement] for widgets that wrap other widgets only to provide
	//     additional data, but otherwise act transparently. [Flexible] and
	//     [KeyedWidget] are two examples.
	//   - [NewInheritedElement]
	//
	// See the documentation on [Element] for more information about the element
	// tree.
	CreateElement() Element
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
	Handle() *ElementHandle
	Transition(t ElementTransition)
	PerformRebuild()
}

type ElementWithChildren interface {
	Element
	Children() iter.Seq[Element]
}

type InteriorElement interface {
	ParentElement
	Build() Widget
}

func DidChangeDependencies(el Element) {
	MarkNeedsBuild(el)
	el.Transition(ElementTransition{Kind: ElementChangedDependencies})
}

func Update(el Element, newWidget Widget) {
	h := el.Handle()
	oldWidget := h.widget
	h.widget = newWidget
	if pd, ok := h.widget.(ParentDataWidget); ok {
		ApplyParentData(pd, el)
	}
	for dependent := range h.dependents {
		// OPT(dh): introduce UpdateShouldNotify
		DidChangeDependencies(dependent)
	}
	el.Transition(ElementTransition{Kind: ElementUpdated, OldWidget: oldWidget})
}

func RenderObjectAttachingChild(el Element) Element {
	if _, ok := el.(RenderObjectElement); ok {
		return nil
	}
	var out Element
	if el, ok := el.(ElementWithChildren); ok {
		for child := range el.Children() {
			debug.Assert(out == nil)
			out = child
		}
	}
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
	if el, ok := el.(ElementWithChildren); ok {
		for child := range el.Children() {
			AttachRenderObject(child, slot)
		}
	}
	el.Handle().slot = slot
}

type RenderObjectDetacher interface {
	PerformDetachRenderObject()
}

// DetachRenderObject recursively instructs the children of the element to detach their render object.
// Elements that implement RenderObjectDetacher have their AfterDetachRenderObject method called instead.
func DetachRenderObject(el Element) {
	if el, ok := el.(RenderObjectDetacher); ok {
		el.PerformDetachRenderObject()
		return
	}
	if el, ok := el.(ElementWithChildren); ok {
		for child := range el.Children() {
			DetachRenderObject(child)
		}
	}
	el.Handle().slot = int(math.MinInt)
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
	debug.Assert(el.Handle().lifecycleState == ElementLifecycleInactive)
	hadDependencies := len(el.Handle().dependencies) != 0 || el.Handle().hadUnsatisfiedDependencies

	h := el.Handle()
	h.lifecycleState = ElementLifecycleActive
	// We unregistered our dependencies in deactivate, but never cleared the list.
	// Since we're going to be reused, let's clear our list now.
	clear(el.Handle().dependencies)
	el.Handle().hadUnsatisfiedDependencies = false
	updateInheritance(el)
	// el.attachNotificationTree()
	if h.dirty {
		h.BuildOwner.scheduleBuildFor(el)
	}
	if hadDependencies {
		DidChangeDependencies(el)
	}

	el.Transition(ElementTransition{Kind: ElementActivated})
}

func Deactivate(el Element) {
	el.Transition(ElementTransition{Kind: ElementDeactivating})

	for dependency := range el.Handle().dependencies {
		delete(dependency.Handle().dependents, el)
		// For expediency, we don't actually clear the list here, even though it's
		// no longer representative of what we are registered with. If we never
		// get re-used, it doesn't matter. If we do, then we'll clear the list in
		// activate(). The benefit of this is that it allows Element's activate()
		// implementation to decide whether to rebuild based on whether we had
		// dependencies here.
	}
	el.Handle().inheritedElements = nil

	el.Handle().lifecycleState = ElementLifecycleInactive
}

func Mount(el, parent Element, newSlot int) {
	h := el.Handle()
	h.parent = parent
	h.slot = newSlot
	h.lifecycleState = ElementLifecycleActive
	if parent != nil {
		h.depth = parent.Handle().depth + 1
	} else {
		h.depth = 1
	}
	if parent != nil {
		// Only assign ownership if the parent is non-null. If parent is null
		// (the root node), the owner should have already been assigned.
		h.BuildOwner = parent.Handle().BuildOwner
	}

	if widget, ok := h.widget.(KeyedWidget); ok {
		if key, ok := widget.GetKey().(GlobalKey); ok {
			h.BuildOwner.RegisterGlobalKey(key, el)
		}
	}

	updateInheritance(el)

	// XXX attachNotificationTree

	el.Handle().dirty = true
	el.Transition(ElementTransition{Kind: ElementMounted, Parent: parent, NewSlot: newSlot})
}

// Unmount unmounts the element.
func Unmount(el Element) {
	h := el.Handle()
	if keyer, ok := h.widget.(KeyedWidget); ok {
		if key, ok := keyer.GetKey().(GlobalKey); ok {
			h.BuildOwner.UnregisterGlobalKey(key, el)
		}
	}
	h.lifecycleState = ElementLifecycleDefunct

	el.Transition(ElementTransition{Kind: ElementUnmounted})
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

type ParentElement interface {
	ElementWithChildren
	// XXX figure out a better API
	GetChildren() []Element
	SetChildren(children []Element)
	ForgottenChildren() map[Element]struct{}
}

type WidgetBuilder interface {
	Build(ctx BuildContext) Widget
}

var _ RenderObjectElement = (*SimpleRenderObjectElement)(nil)

type SimpleRenderObjectElement struct {
	RenderObjectElementHandle
	ManyChildElements
}

// AttachRenderObject implements RenderObjectElement.
func (el *SimpleRenderObjectElement) AttachRenderObject(slot int) {
	RenderObjectElementAttachRenderObject(el, slot)
}

// InsertRenderObjectChild implements RenderObjectElement.
func (el *SimpleRenderObjectElement) InsertRenderObjectChild(child render.Object, slot int) {
	RenderObjectElementInsertRenderObjectChild(el, child, slot)
}

// MoveRenderObjectChild implements RenderObjectElement.
func (el *SimpleRenderObjectElement) MoveRenderObjectChild(child render.Object, newSlot int) {
	RenderObjectElementMoveRenderObjectChild(el, child, newSlot)
}

// RemoveRenderObjectChild implements RenderObjectElement.
func (el *SimpleRenderObjectElement) RemoveRenderObjectChild(child render.Object, slot int) {
	RenderObjectElementRemoveRenderObjectChild(el, child, slot)
}

// PerformRebuild implements RenderObjectElement.
func (el *SimpleRenderObjectElement) PerformRebuild() {
	RenderObjectElementPerformRebuild(el)
}

// Transition implements RenderObjectElement.
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

func (o *BuildOwner) RegisterGlobalKey(key GlobalKey, el Element) {
	o.globals[key] = el
}

func (o *BuildOwner) UnregisterGlobalKey(key GlobalKey, el Element) {
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
	if el.Handle().inDirtyList {
		o.dirtyElementsNeedsResorting = true
		return
	}
	if !o.inDrawFrame && !o.scheduledFlushDirtyElements && o.OnBuildScheduled != nil {
		o.scheduledFlushDirtyElements = true
		o.OnBuildScheduled()
	}
	o.dirtyElements = append(o.dirtyElements, el)
	el.Handle().inDirtyList = true
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

//go:generate stringer -type=Lifecycle --trimprefix=ElementLifecycle
type Lifecycle uint8

const (
	ElementLifecycleIdle Lifecycle = iota
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
	lifecycleState Lifecycle
	depth          int
	BuildOwner     *BuildOwner
	dirty          bool
	inDirtyList    bool
	widget         Widget
	// OPT(dh): use a persistent data structure for inheritedElements
	inheritedElements          map[reflect.Type]InheritedElement
	dependencies               map[InheritedElement]struct{}
	dependents                 map[Element]struct{}
	hadUnsatisfiedDependencies bool
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

func (el *RenderObjectElementHandle) AfterUpdateSlot(oldSlot, newSlot int) {
	if ancestor := el.ancestorRenderObjectElement; ancestor != nil {
		ancestor.MoveRenderObjectChild(el.RenderObject, el.slot)
	}
}

func (el *RenderObjectElementHandle) PerformDetachRenderObject() {
	if el.ancestorRenderObjectElement != nil {
		el.ancestorRenderObjectElement.RemoveRenderObjectChild(el.RenderObject, el.slot)
		el.ancestorRenderObjectElement = nil
	}
	el.slot = -1
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
	if el, ok := el.(ElementWithChildren); ok {
		for child := range el.Children() {
			els.unmount(child)
		}
	}
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
	if el, ok := el.(ElementWithChildren); ok {
		for child := range el.Children() {
			els.deactivateRecursively(child)
		}
	}
}

func (els *inactiveElements) add(el Element) {
	debug.Assert(!els.locked)
	_, ok := els.elements[el]
	debug.Assert(!ok)
	debug.Assert(el.Handle().parent == nil)
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

func InflateWidget(parent Element, widget Widget, newSlot int) Element {
	if widget, ok := widget.(KeyedWidget); ok {
		if key, ok := widget.GetKey().(GlobalKey); ok {
			newChild := RetakeInactiveElement(parent, key, widget)
			if newChild != nil {
				activateWithParent(newChild, parent, newSlot)
				updatedChild := UpdateChild(parent, newChild, widget, newSlot)
				debug.Assert(newChild == updatedChild)
				return updatedChild
			}
		}
	}
	newChild := widget.CreateElement()
	Mount(newChild, parent, newSlot)

	return newChild
}

func RetakeInactiveElement(el Element, key GlobalKey, newWidget Widget) Element {
	// The "inactivity" of the element being retaken here may be forward-looking: if
	// we are taking an element with a GlobalKey from an element that currently has
	// it as a child, then we know that element will soon no longer have that
	// element as a child. The only way that assumption could be false is if the
	// global key is being duplicated, and we'll try to track that using the
	// _debugTrackElementThatWillNeedToBeRebuiltDueToGlobalKeyShenanigans call below.

	element := el.Handle().BuildOwner.globals[key]
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
	debug.Assert(el.Handle().lifecycleState == ElementLifecycleInactive)
	el.Handle().parent = parent
	updateDepth(el, parent.Handle().depth)
	activateRecursively(el)
	AttachRenderObject(el, newSlot)
	debug.Assert(el.Handle().lifecycleState == ElementLifecycleActive)
}

func updateDepth(el Element, parentDepth int) {
	expectedDepth := parentDepth + 1
	if el.Handle().depth < expectedDepth {
		el.Handle().depth = expectedDepth
		if el, ok := el.(ElementWithChildren); ok {
			for child := range el.Children() {
				updateDepth(child, expectedDepth)
			}
		}
	}
}

func activateRecursively(el Element) {
	debug.Assert(el.Handle().lifecycleState == ElementLifecycleInactive)
	Activate(el)
	debug.Assert(el.Handle().lifecycleState == ElementLifecycleActive)
	if el, ok := el.(ElementWithChildren); ok {
		for child := range el.Children() {
			activateRecursively(child)
		}
	}
}

func updateSlotForChild(el, child Element, newSlot int) {
	for child != nil {
		UpdateSlot(child, newSlot)
		child = RenderObjectAttachingChild(child)
	}
}

func rebuild(el Element) {
	if el.Handle().lifecycleState != ElementLifecycleActive || !el.Handle().dirty {
		return
	}
	el.PerformRebuild()
	el.Handle().dirty = false
}

func forceRebuild(el Element) {
	if el.Handle().lifecycleState != ElementLifecycleActive {
		return
	}
	el.PerformRebuild()
	el.Handle().dirty = false
}

func UpdateChildren(el ParentElement, newWidgets []Widget) []Element {
	oldChildren := el.GetChildren()
	replaceWithNilIfForgotten := func(child Element) Element {
		if _, ok := el.ForgottenChildren()[child]; ok {
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
		if _, ok := el.ForgottenChildren()[oldChild]; !ok {
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
		} else if child, ok := child.(ElementWithChildren); ok {
			for child2 := range child.Children() {
				applyParentData(child2)
			}
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

func WidgetChildrenIter(parent Widget) iter.Seq2[int, Widget] {
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
		return nil
	}
}

type SingleChildElement struct {
	child [1]Element
}

func (s *SingleChildElement) GetChildren() []Element {
	if s.child[0] == nil {
		return nil
	} else {
		return s.child[:]
	}
}

func (s *SingleChildElement) ForgottenChildren() map[Element]struct{} {
	return nil
}

func (s *SingleChildElement) SetChildren(children []Element) {
	debug.Assert(len(children) < 2)
	if len(children) == 0 {
		s.child[0] = nil
	} else {
		s.child[0] = children[0]
	}
}

func (s *SingleChildElement) Children() iter.Seq[Element] {
	return func(yield func(Element) bool) {
		if s.child[0] == nil {
			return
		}
		yield(s.child[0])
	}
}

func (s *SingleChildElement) Child() Element {
	return s.child[0]
}

func (s *SingleChildElement) SetChild(child Element) {
	s.child[0] = child
}

func (s *SingleChildElement) ForgetChild(child Element) {
	debug.Assert(s.child[0] == child)
	s.child[0] = nil
}

type ManyChildElements struct {
	children          []Element
	forgottenChildren map[Element]struct{}
}

func (m *ManyChildElements) SetChildren(children []Element) {
	m.children = children
	clear(m.forgottenChildren)
}

func (m *ManyChildElements) Children() iter.Seq[Element] {
	return func(yield func(Element) bool) {
		forgotten := m.forgottenChildren
		for _, child := range m.children {
			if _, ok := forgotten[child]; !ok {
				if !yield(child) {
					break
				}
			}
		}
	}
}

func (m *ManyChildElements) GetChildren() []Element {
	return m.children
}

func (m *ManyChildElements) ForgottenChildren() map[Element]struct{} {
	return m.forgottenChildren
}

func (m *ManyChildElements) ForgetChild(child Element) {
	if m.forgottenChildren == nil {
		m.forgottenChildren = make(map[Element]struct{})
	}
	m.forgottenChildren[child] = struct{}{}
}

func NewRenderObjectElement(w RenderObjectWidget) *SimpleRenderObjectElement {
	el := &SimpleRenderObjectElement{}
	el.widget = w
	return el
}

var _ Widget = (*MediaQuery)(nil)

type MediaQuery struct {
	Data  MediaQueryData
	Child Widget
}

func (m *MediaQuery) CreateElement() Element {
	return NewInheritedElement(m)
}

type MediaQueryData struct {
	Scale float64
	Size  curve.Size
}
