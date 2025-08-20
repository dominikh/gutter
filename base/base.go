// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package base

import "honnef.co/go/stuff/container/maybe"

type Listener uint64

type Listenable interface {
	AddListener(cb func()) Listener
	RemoveListener(l Listener)
	ClearListeners()
}

type ValueListenable[T any] interface {
	Listenable
	Value() maybe.Option[T]
}

var _ Listenable = (*PlainListenable)(nil)

type PlainListenable struct {
	maxID     Listener
	listeners map[Listener]func()
}

func (l *PlainListenable) AddListener(cb func()) Listener {
	l.maxID++
	if l.listeners == nil {
		l.listeners = make(map[Listener]func())
	}
	l.listeners[l.maxID] = cb
	return l.maxID
}

func (l *PlainListenable) RemoveListener(id Listener) {
	delete(l.listeners, id)
}

func (l *PlainListenable) ClearListeners() {
	clear(l.listeners)
}

func (l *PlainListenable) NotifyListeners() {
	for _, cb := range l.listeners {
		cb()
	}
}
