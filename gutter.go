package main

// We don't have const constructors, which hinders subtree reuse

// We don't have optional arguments

// We don't have subclassing

// We don't have getters or setters

type BuildContext struct{}

type BuilderWidget[W Widget] struct {
	Builder func(BuildContext) W
}

func Builder[W Widget](b func(ctx BuildContext) W) BuilderWidget[W] {
	return BuilderWidget[W]{Builder: b}
}

func main() {
	var state struct {
		beAwesome TrivialExampleState[bool]
	}

	_ = Toplevel(
		Row(
			Depends(&state.beAwesome, func(beAwesome State[bool]) Checkbox {
				return Checkbox{
					Value:    beAwesome.Get(),
					OnToggle: func(b bool) { beAwesome.Set(b) },
				}
			}),

			Reads(&state.beAwesome, func(beAwesome ReadableState[bool]) Checkbox {
				return Checkbox{
					Value:    beAwesome.Get(), // this checkbox is synced with the first checkbox
					OnToggle: func(b bool) {},
				}
			}),
		),
	)
}

type Widget interface{}

type Checkbox struct {
	Value    bool
	OnToggle func(b bool)
}

type ToplevelWidget[W Widget] struct {
	Child W
}

func Toplevel[W Widget](w W) ToplevelWidget[W] {
	return ToplevelWidget[W]{w}
}

type ReadableState[T any] interface {
	Get() T
	Generation() uint64
}

type WritableState[T any] interface {
	Set(T)
}

type State[T any] interface {
	ReadableState[T]
	WritableState[T]
}

type TrivialExampleState[T any] struct {
	Value T

	generation uint64
}

func (s TrivialExampleState[T]) Get() T {
	return s.Value
}

func (s *TrivialExampleState[T]) Set(v T) {
	s.Value = v
	s.generation++
}

func (s *TrivialExampleState[T]) Generation() uint64 {
	return s.generation
}

type ReadValue[T any, S ReadableState[T]] struct {
	Value S
}

type RowWidget struct {
	Children []Widget
}

func Row(widgets ...Widget) RowWidget {
	return RowWidget{Children: widgets}
}

type ReadWidget[T any, S ReadableState[T], W Widget] struct {
	Dependency S
	Callback   func(s ReadableState[T]) W
}

type WriteWidget[T any, S WritableState[T], W Widget] struct {
	Dependency S
	Callback   func(s WritableState[T]) W
}

type ReadWriteWidget[T any, S State[T], W Widget] struct {
	Dependency S
	Callback   func(s State[T]) W
}

func Reads[T any, S ReadableState[T], W Widget](s S, cb func(s ReadableState[T]) W) ReadWidget[T, S, W] {
	return ReadWidget[T, S, W]{s, cb}
}

func Writes[T any, S WritableState[T], W Widget](s S, cb func(s WritableState[T]) W) WriteWidget[T, S, W] {
	return WriteWidget[T, S, W]{s, cb}
}

func Depends[T any, S State[T], W Widget](s S, cb func(s State[T]) W) ReadWriteWidget[T, S, W] {
	return ReadWriteWidget[T, S, W]{s, cb}
}
