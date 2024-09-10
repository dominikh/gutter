// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package animation

import (
	"time"

	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/maybe"
)

// TODO(dh): move Listener stuff to a different package

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

type StatusListener uint64

type StatusListenable interface {
	AddStatusListener(cb func(status AnimationStatus)) StatusListener
	RemoveStatusListener(l StatusListener)
	ClearStatusListeners()
}

type PlainStatusListenable struct {
	maxID     StatusListener
	listeners map[StatusListener]func(AnimationStatus)
}

func (l *PlainStatusListenable) AddStatusListener(cb func(status AnimationStatus)) StatusListener {
	l.maxID++
	if l.listeners == nil {
		l.listeners = make(map[StatusListener]func(AnimationStatus))
	}
	l.listeners[l.maxID] = cb
	return l.maxID
}

func (l *PlainStatusListenable) RemoveStatusListener(id StatusListener) {
	delete(l.listeners, id)
}

func (l *PlainStatusListenable) NotifyStatusListeners(status AnimationStatus) {
	for _, cb := range l.listeners {
		cb(status)
	}
}

func (l *PlainStatusListenable) ClearStatusListeners() {
	clear(l.listeners)
}

// A TickerCallback gets called by a [Ticker]. The argument is the time elapsed
// since the ticker was started.
type TickerCallback func(t time.Duration)

// A TickerProvider is responsible for creating tickers.
type TickerProvider interface {
	CreateTicker(cb TickerCallback) Ticker
}

// PlainTickerProvider creates tickers backed by frame callbacks.
type PlainTickerProvider struct {
	// TODO(dh): implement a ticker provider that supports something akin to
	// Flutter's TickerMode, i.e. a way to disable tickers in widget subtrees.
	FrameCallbacker FrameCallbacker
}

func (p *PlainTickerProvider) CreateTicker(cb TickerCallback) Ticker {
	return &PlainTicker{
		OnTick:          cb,
		FrameCallbacker: p.FrameCallbacker,
	}
}

// A Ticker periodically calls a user-provided callback, passing it the amount
// of time that has passed since the ticker was started. Conceptually, tickers
// fire once at the start of every frame, but tests and debugging utilities may
// adjust this to fake the passage of time.
//
// A ticker will start invoking callbacks once it has been started, and will
// cease once it has been stopped. A ticker may be muted, in which case it still
// perceives the passage of time, but doesn't invoke any callbacks. This can be
// used to, for example, avoid updating animations while a widget isn't visible.
//
// Ticker instances are created by [TickerProvider] implementations. Tickers should be returned
// in the stopped state.
//
// XXX point to the default ticker provider.
type Ticker interface {
	// SetMuted changes the ticker's muted state.
	SetMuted(muted bool)
	// Active reports whether the ticker has been started.
	Active() bool
	// Ticking reports whether the ticker is active and hasn't been muted.
	Ticking() bool
	// Muted reports whether the ticker has been muted.
	Muted() bool
	// Start starts the ticker.
	Start()
	// Stop stops the ticker.
	Stop()

	Dispose()
}

type FrameCallback func(d time.Duration)

type FrameCallbacker interface {
	ScheduleFrameCallback(cb FrameCallback) uint64
	CancelFrameCallback(id uint64)
}

// PlainTicker is a [Ticker] that calls OnTick once per frame, driven by a
// [FrameCallbacker].
type PlainTicker struct {
	OnTick          TickerCallback
	FrameCallbacker FrameCallbacker

	animationID maybe.Option[uint64]
	active      bool
	muted       bool
	startTime   time.Duration

	// A cached version of the PlainTicker.tick method value to avoid
	// allocations.
	tickFn func(time.Duration)
}

// SetMuted implements [Ticker.SetMuted].
func (t *PlainTicker) SetMuted(muted bool) {
	if t.muted == muted {
		return
	}
	t.muted = muted
	if muted {
		t.unscheduleTick()
	} else if t.shouldScheduleTick() {
		t.scheduleTick()
	}
}

// Active implements [Ticker.Active].
func (t *PlainTicker) Active() bool {
	return t.active
}

// Ticking implements [Ticker.Ticking].
func (t *PlainTicker) Ticking() bool {
	return t.active && !t.muted
}

// Muted implements [Ticker.Muted].
func (t *PlainTicker) Muted() bool {
	return t.muted
}

func (t *PlainTicker) scheduled() bool {
	_, ok := t.animationID.Get()
	return ok
}

func (t *PlainTicker) scheduleTick() {
	if t.tickFn == nil {
		t.tickFn = t.tick
	}
	t.animationID = maybe.Some(t.FrameCallbacker.ScheduleFrameCallback(t.tickFn))
}

func (t *PlainTicker) unscheduleTick() {
	if t.scheduled() {
		t.FrameCallbacker.CancelFrameCallback(t.animationID.Unwrap())
		t.animationID.Clear()
	}
	debug.Assert(!t.shouldScheduleTick())
}

func (t *PlainTicker) shouldScheduleTick() bool {
	return !t.muted && t.active && !t.scheduled()
}

func (t *PlainTicker) tick(now time.Duration) {
	debug.Assert(t.Ticking())
	debug.Assert(t.scheduled())
	t.animationID.Clear()

	// TODO(dh): this causes the ticker to lag behind by one frame. We'd need
	// the current frame time in Start to avoid that, but that'd require a lot
	// of plumbing to get the time into Start.
	if t.startTime == 0 {
		t.startTime = now
	}

	t.OnTick(now - t.startTime)

	// The onTick callback may have scheduled another tick already, for
	// example by calling stop then start again.
	if t.shouldScheduleTick() {
		t.scheduleTick()
	}
}

// Start implements [Ticker.Start].
func (t *PlainTicker) Start() {
	debug.Assert(!t.active)
	t.active = true
	t.startTime = 0
	if t.shouldScheduleTick() {
		t.scheduleTick()
	}
}

// Stop implements [Ticker.Stop].
func (t *PlainTicker) Stop() {
	if !t.active {
		return
	}

	t.active = false
	t.unscheduleTick()
}

func (t *PlainTicker) Dispose() {
	t.Stop()
}
