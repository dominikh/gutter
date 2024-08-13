// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package maybe

import (
	"fmt"

	"github.com/go-json-experiment/json"
)

type Option[T any] struct {
	set   bool
	value T
}

func (o Option[T]) String() string {
	if !o.set {
		return "None"
	}
	return fmt.Sprintf("Some(%v)", o.value)
}

func (o Option[T]) Set() bool {
	return o.set
}

func (o Option[T]) Get() (T, bool) {
	return o.value, o.set
}

func (o Option[T]) Unwrap() T {
	if !o.set {
		panic("option isn't set")
	}
	return o.value
}

func (o Option[T]) UnwrapOr(def T) T {
	if o.set {
		return o.value
	} else {
		return def
	}
}

func (o *Option[T]) Take() Option[T] {
	v := *o
	*o = Option[T]{}
	return v
}

func Some[T any](v T) Option[T] {
	return Option[T]{
		set:   true,
		value: v,
	}
}

func (o *Option[T]) UnmarshalJSON(b []byte) error {
	var v *T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v == nil {
		*o = Option[T]{}
	} else {
		o.set = true
		o.value = *v
	}
	return nil
}

func Map[T1, T2 any](o Option[T1], fn func(T1) T2) Option[T2] {
	if !o.set {
		return Option[T2]{}
	}
	return Option[T2]{
		set:   true,
		value: fn(o.value),
	}
}
