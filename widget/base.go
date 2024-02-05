package widget

import (
	"slices"
	"unsafe"

	"honnef.co/go/gutter/render"
)

// TODO Flutter has createElement() => StatelessElement(this) for StatelessWidget, which differs from the
// createElement for Widget. Do we need that? What does StatelessElement do and look like?

// TODO implement support for stateful widgets

// TODO MediaQuery
// TODO support inheritance (cf inheritedElements in framework.dart)
// TODO support "Notification"
// TODO support global keys

type Widget interface {
	Key() any

	// Most implementations want to call one of
	// - NewSingleChildRenderObjectElement
	CreateElement() Element
}

type StatelessWidget interface {
	Widget
	Build(ctx BuildContext) Widget
}

// All implementations of Element must embed ElementHandle. All elements that represent render objects must
// embed RenderObjectElementHandle, which itself embeds ElementHandle.
type Element interface {
	// Most implementations want to call one of
	// - ElementUpdateChild
	// or embed one of
	// - SingleChildRenderObjectElement
	UpdateChild(child Element, newWidget Widget, newSlot any) Element

	// Most implementations want to call one of
	// - ElementUpdate
	// - StatelessElementUpdate
	// - RenderObjectElementUpdate
	// - SingleChildRenderObjectElementUpdate
	// or embed one of
	// - SingleChildRenderObjectElement
	//
	// Custom implementations must call one of the aforementioned functions.
	Update(newWidget Widget)

	// Provided by default when embedding ElementHandle.
	Handle() *ElementHandle
	Parent() Element
	Slot() any

	// Most implementations want to call one of
	// - ElementActivate
	// or embed one of
	// - SingleChildRenderObjectElement
	//
	// Custom implementations must call ElementActivate.
	Activate()

	// Provided by default when embedding ElementHandle. Custom implementations must call ElementDeactivate.
	Deactivate()

	// Most implementations want to call one of
	// - ElementMount
	// - RenderObjectElementMount
	// or embed one of
	// - SingleChildRenderObjectElement
	Mount(parent Element, slot any)

	// Most implementations want to call one of
	// - ElementUnmount
	// - RenderObjectElementUnmount
	// or embed one of
	// - SingleChildRenderObjectElement
	Unmount()

	// Most implementations want to call one of
	// - ElementAttachRenderObject
	// - RenderObjectElementAttachRenderObject
	// or embed one of
	// - SingleChildRenderObjectElement
	AttachRenderObject(slot any)

	// Most implementations want to call one of
	// - ElementDetachRenderObject
	// - RenderObjectElementDetachRenderObject
	// or embed one of
	// - SingleChildRenderObjectElement
	DetachRenderObject()

	// Provided by default when embedding ElementHandle or RenderObjectElementHandle.
	// Custom implementations must call ElementPerformRebuild or RenderObjectElementPerformRebuild.
	PerformRebuild()

	// Either implement it or embed one of
	// - SingleChildRenderObjectElement
	VisitChildren(yield func(el Element) bool)

	// Most implementations want to call one of
	// - ElementRenderObjectAttachingChild
	// - RenderObjectElementRenderObjectAttachingChild
	// or embed one of
	// - SingleChildRenderObjectElement
	RenderObjectAttachingChild() Element

	// Provided by default when embedding ElementHandle or RenderObjectElementHandle.
	// Custom implementations must call ElementUpdateSlot or RenderObjectElementUpdateSlot.
	UpdateSlot(newSlot any)
}

type BuildContext interface {
}

type RenderObjectWidget interface {
	Widget
	CreateRenderObject(ctx BuildContext) render.Object
	UpdateRenderObject(ctx BuildContext, obj render.Object)
}

type RenderObjectElement interface {
	Element

	// Provided by default when embedding RenderObjectElementHandle.
	AncestorRenderObjectElement() RenderObjectElement
	RenderHandle() *RenderObjectElementHandle

	// Most implementations want to embed one of
	// - SingleChildRenderObjectElement
	InsertRenderObjectChild(child render.Object, slot any)
	RemoveRenderObjectChild(child render.Object, slot any)
	MoveRenderObjectChild(child render.Object, oldSlot, newSlot any)
}

type ChildForgetter interface {
	ForgetChild(child Element)
}

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

func (h *ElementHandle) Slot() any       { return h.slot }
func (h *ElementHandle) PerformRebuild() { h.dirty = false }
func (h *ElementHandle) Parent() Element { return h.parent }

func ElementMount(el Element, parent Element, newSlot any) {
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
		// See RootRenderObjectElement.assignOwner().
		h.owner = parent.Handle().owner
	}
}

func ElementUnmount(el Element) {
	key := el.Handle().widget.Key()
	if key, ok := key.(*GlobalKey); ok {
		_ = key
		// owner!._unregisterGlobalKey(key, this); // XXX
	}
	h := el.Handle()
	h.lifecycleState = ElementLifecycleDefunct
}

func ElementAttachRenderObject(el Element, slot any) {
	el.VisitChildren(func(child Element) bool {
		child.AttachRenderObject(slot)
		return true
	})
	el.Handle().slot = slot
}

func ElementPerformRebuild(el Element) {
	el.Handle().PerformRebuild()
}

func RenderObjectElementPerformRebuild(el RenderObjectElement) {
	el.RenderHandle().PerformRebuild()
}

type RenderObjectElementHandle struct {
	ElementHandle
	renderObject                render.Object
	ancestorRenderObjectElement RenderObjectElement
}

func (el *RenderObjectElementHandle) PerformRebuild() {
	el.widget.(RenderObjectWidget).UpdateRenderObject(el, el.RenderHandle().renderObject)
	el.ElementHandle.PerformRebuild()
}

func (el *RenderObjectElementHandle) RenderHandle() *RenderObjectElementHandle {
	return el
}

func (h *RenderObjectElementHandle) RenderObject() render.Object {
	return h.renderObject
}

func (h *RenderObjectElementHandle) AncestorRenderObjectElement() RenderObjectElement {
	return h.ancestorRenderObjectElement
}

func RenderObjectElementMount(el RenderObjectElement, parent Element, slot any) {
	ElementMount(el, parent, slot)

	el.RenderHandle().renderObject = el.Handle().widget.(RenderObjectWidget).CreateRenderObject(el)
	el.AttachRenderObject(slot)
	el.PerformRebuild() // clears the "dirty" flag
}

type RenderObjectUnmountNotifyee interface {
	DidUnmountRenderObject(obj render.Object)
}

func RenderObjectElementUnmount(el RenderObjectElement) {
	oldWidget := el.Handle().widget.(RenderObjectWidget)
	ElementUnmount(el)
	h := el.RenderHandle()
	if n, ok := oldWidget.(RenderObjectUnmountNotifyee); ok {
		n.DidUnmountRenderObject(h.RenderObject())
	}
	render.Dispose(h.renderObject)
	h.renderObject = nil
}

func RenderObjectElementAttachRenderObject(el Element, slot any) {
	he := el.Handle()
	he.slot = slot
	hr := el.(RenderObjectElement).RenderHandle()
	hr.ancestorRenderObjectElement = findAncestorRenderObjectElement(el.(RenderObjectElement))
	if hr.ancestorRenderObjectElement != nil {
		hr.ancestorRenderObjectElement.InsertRenderObjectChild(hr.renderObject, slot)
	}
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
		// If the `dirty` values are not equal, sort with non-dirty elements being
		// less than dirty elements.
		isBDirty := bh.dirty
		if ah.dirty != isBDirty {
			if isBDirty {
				return -1
			} else {
				return 1
			}
		}
		// Otherwise, `depth`s and `dirty`s are equal.
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

func ElementDetachRenderObject(el Element) {
	el.VisitChildren(func(el Element) bool {
		el.DetachRenderObject()
		return true
	})
	el.Handle().slot = nil
}

func RenderObjectElementDetachRenderObject(el RenderObjectElement) {
	h := el.RenderHandle()
	if h.ancestorRenderObjectElement != nil {
		h.ancestorRenderObjectElement.RemoveRenderObjectChild(h.renderObject, h.slot)
		h.ancestorRenderObjectElement = nil
	}
	el.Handle().slot = nil
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

func ElementActivate(el Element) {
	// hadDependencies := (el._dependencies != null && el._dependencies.isNotEmpty) || el._hadUnsatisfiedDependencies // XXX implement once we have InheritedWidget
	el.Handle().lifecycleState = ElementLifecycleActive
	// We unregistered our dependencies in deactivate, but never cleared the list.
	// Since we're going to be reused, let's clear our list now.
	// XXX
	// if el._dependencies != nil {
	// 	el._dependencies.clear()
	// }
	// el._hadUnsatisfiedDependencies = false
	// el._updateInheritance()
	// el.attachNotificationTree()
	if el.Handle().dirty {
		el.Handle().owner.scheduleBuildFor(el)
	}
	// if hadDependencies {
	// 	el.didChangeDependencies()
	// }
}

// Update the given child with the given new configuration.
//
// This method is the core of the widgets system. It is called each time we
// are to add, update, or remove a child based on an updated configuration.
//
// The `newSlot` argument specifies the new value for this element's [slot].
//
// If the `child` is null, and the `newWidget` is not null, then we have a new
// child for which we need to create an [Element], configured with `newWidget`.
//
// If the `newWidget` is null, and the `child` is not null, then we need to
// remove it because it no longer has a configuration.
//
// If neither are null, then we need to update the `child`'s configuration to
// be the new configuration given by `newWidget`. If `newWidget` can be given
// to the existing child (as determined by [Widget.canUpdate]), then it is so
// given. Otherwise, the old child needs to be disposed and a new child
// created for the new configuration.
//
// If both are null, then we don't have a child and won't have a child, so we
// do nothing.
//
// The [updateChild] method returns the new child, if it had to create one,
// or the child that was passed in, if it just had to update the child, or
// null, if it removed the child and did not replace it.
//
// The following table summarizes the above:
//
// |                     | **newWidget == null**  | **newWidget != null**   |
// | :-----------------: | :--------------------- | :---------------------- |
// |  **child == null**  |  Returns null.         |  Returns new [Element]. |
// |  **child != null**  |  Old child is removed, returns null. | Old child updated if possible, returns child or new [Element]. |
//
// The `newSlot` argument is used only if `newWidget` is not null. If `child`
// is null (or if the old child cannot be updated), then the `newSlot` is
// given to the new [Element] that is created for the child, via
// [inflateWidget]. If `child` is not null (and the old child _can_ be
// updated), then the `newSlot` is given to [updateSlotForChild] to update
// its slot, in case it has moved around since it was last built.
//
// See the [RenderObjectElement] documentation for more information on slots.
func ElementUpdateChild(el, child Element, newWidget Widget, newSlot any) Element {
	if newWidget == nil {
		if child != nil {
			deactivateChild(child)
		}
		return nil
	}

	var newChild Element
	if child != nil {
		if child.Handle().widget == newWidget {
			// We don't insert a timeline event here, because otherwise it's
			// confusing that widgets that "don't update" (because they didn't
			// change) get "charged" on the timeline.
			if child.Handle().slot != newSlot {
				updateSlotForChild(el, child, newSlot)
			}
			newChild = child
		} else if canUpdate(child.Handle().widget, newWidget) {
			if child.Handle().slot != newSlot {
				updateSlotForChild(el, child, newSlot)
			}
			child.Update(newWidget)
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

// Change the slot that the given child occupies in its parent.
//
// Called by [MultiChildRenderObjectElement], and other [RenderObjectElement]
// subclasses that have multiple children, when child moves from one position
// to another in this element's child list.
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

func ElementRenderObjectAttachingChild(el Element) Element {
	var out Element
	el.VisitChildren(func(child Element) bool {
		out = child
		return false
	})
	return out
}

func RenderObjectElementRenderObjectAttachingChild(el Element) Element {
	return nil
}

func ElementUpdate(el Element, newWidget Widget) {
	el.Handle().widget = newWidget
}

func StatelessElementUpdate(el Element, newWidget StatelessWidget) {
	ElementUpdate(el, newWidget)
	forceRebuild(el)
}

func RenderObjectElementUpdate(el Element, newWidget RenderObjectWidget) {
	ElementUpdate(el, newWidget)
	el.PerformRebuild()
}

func SingleChildRenderObjectElementUpdate(el SingleChildElement, newWidget RenderObjectWidget) {
	RenderObjectElementUpdate(el, newWidget)

	el.SetChild(el.UpdateChild(el.Child(), el.Handle().widget.(SingleChildWidget).Child(), nil))
}

type SingleChildWidget interface {
	Widget
	Child() Widget
}

type SingleChildElement interface {
	Element
	Child() Element
	SetChild(child Element)
}

func ElementUpdateSlot(el Element, newSlot any) {
	el.Handle().UpdateSlot(newSlot)
}

func RenderObjectElementUpdateSlot(el RenderObjectElement, newSlot any) {
	oldSlot := el.Handle().slot
	ElementUpdateSlot(el, newSlot)
	if ancestor := el.AncestorRenderObjectElement(); ancestor != nil {
		ancestor.MoveRenderObjectChild(el.RenderHandle().renderObject, oldSlot, el.Handle().slot)
	}
}

func (h *ElementHandle) UpdateSlot(newSlot any) {
	h.slot = newSlot
}

func (h *RenderObjectElementHandle) UpdateSlot(newSlot any) {
	oldSlot := h.slot
	h.slot = newSlot
	if ancestor := h.ancestorRenderObjectElement; ancestor != nil {
		ancestor.MoveRenderObjectChild(h.renderObject, oldSlot, h.slot)
	}
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

func ElementDeactivate(el Element) {
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
	el.Handle().Deactivate()
}

func (h *ElementHandle) Deactivate() {
	h.lifecycleState = ElementLifecycleInactive
}

func MarkNeedsBuild(el Element) {
	if el.Handle().lifecycleState != ElementLifecycleActive {
		return
	}
	if el.Handle().dirty {
		return
	}
	el.Handle().dirty = true
	el.Handle().owner.scheduleBuildFor(el)
}
