// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package animation

// TODO(dh): allow waiting for an animation to finish/be cancelled
// TODO(dh): support "flinging" an animation
// TODO(dh): add animateWith

import (
	"fmt"
	"math"
	"time"

	"honnef.co/go/gutter/base"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/jello/jmath"
)

var _ Animation[float64] = (*Controller)(nil)

type animationDirection int

const (
	animationDirectionForward animationDirection = iota
	animationDirectionReverse
)

// TODO(dh): add stringer invocation
type AnimationStatus int

const (
	AnimationStatusDismissed AnimationStatus = iota
	AnimationStatusForward
	AnimationStatusReverse
	AnimationStatusCompleted
)

func (s AnimationStatus) IsForwardOrCompleted() bool {
	switch s {
	case AnimationStatusForward, AnimationStatusCompleted:
		return true
	case AnimationStatusReverse, AnimationStatusDismissed:
		return false
	default:
		panic(fmt.Sprintf("unknown animation status %v", s))
	}
}

type Controller struct {
	LowerBound      float64
	UpperBound      float64
	Duration        time.Duration
	ReverseDuration time.Duration

	ticker              Ticker
	listeners           base.PlainListenable
	statusListeners     PlainStatusListenable
	value               float64
	direction           animationDirection
	status              AnimationStatus
	simulation          Simulation
	lastElapsedDuration time.Duration
	lastReportedStatus  AnimationStatus
}

func NewController(tp TickerProvider) *Controller {
	c := &Controller{
		LowerBound: 0,
		UpperBound: 1,
	}
	c.ticker = tp.CreateTicker(c.tick)
	c.setValue(0)
	return c
}

func (c *Controller) Status() AnimationStatus {
	return c.status
}

func (c *Controller) LastElapsedDuration() time.Duration {
	return c.lastElapsedDuration
}

func (c *Controller) Animating() bool {
	return c.ticker != nil && c.ticker.Active()
}

func (c *Controller) Value() float64 {
	return c.value
}

func (c *Controller) SetValue(v float64) {
	c.Stop()
	c.setValue(v)
	c.notifyListeners()
	c.checkStatusChanged()
}

func (c *Controller) setValue(v float64) {
	c.value = jmath.Clamp(v, c.LowerBound, c.UpperBound)
	switch c.value {
	case c.LowerBound:
		c.status = AnimationStatusDismissed
	case c.UpperBound:
		c.status = AnimationStatusCompleted
	default:
		switch c.direction {
		case animationDirectionForward:
			c.status = AnimationStatusForward
		case animationDirectionReverse:
			c.status = AnimationStatusReverse
		default:
			panic(fmt.Sprintf("internal error: unhandled direction %v", c.direction))
		}
	}
}

func (c *Controller) Reset() {
	c.SetValue(c.LowerBound)
}

func (c *Controller) Forward() {
	c.direction = animationDirectionForward
	c.animateTo(c.UpperBound, nil)
}

func (c *Controller) Reverse() {
	c.direction = animationDirectionReverse
	c.animateTo(c.LowerBound, nil)
}

func (c *Controller) ToggleDirection() {
	if c.status.IsForwardOrCompleted() {
		c.direction = animationDirectionReverse
	} else {
		c.direction = animationDirectionForward
	}
	switch c.direction {
	case animationDirectionForward:
		c.animateTo(c.UpperBound, nil)
	case animationDirectionReverse:
		c.animateTo(c.LowerBound, nil)
	default:
		panic(fmt.Sprintf("internal error: unhandled direction %v", c.direction))
	}
}

func (c *Controller) AnimateTo(v float64, curve Curve) {
	c.direction = animationDirectionForward
	c.animateTo(v, curve)
}

func (c *Controller) AnimateBack(v float64, curve Curve) {
	c.direction = animationDirectionReverse
	c.animateTo(v, curve)
}

func (c *Controller) animateTo(v float64, curve Curve) {
	if curve == nil {
		curve = CurveIdentity
	}

	simulationDuration := c.Duration
	if simulationDuration == 0 {
		rng := c.UpperBound - c.LowerBound
		var remainingFraction float64
		if !math.IsInf(rng, 0) {
			remainingFraction = math.Abs(v-c.value) / rng
		} else {
			remainingFraction = 1
		}
		var directionDuration time.Duration
		if c.direction == animationDirectionReverse && c.ReverseDuration != 0 {
			directionDuration = c.ReverseDuration
		} else {
			directionDuration = c.Duration
		}
		simulationDuration = time.Duration(float64(directionDuration) * remainingFraction)
	} else if v == c.value {
		// Already at v, don't animate.
		simulationDuration = 0
	}
	c.Stop()
	if simulationDuration == 0 {
		if c.value != v {
			c.value = jmath.Clamp(v, c.LowerBound, c.UpperBound)
			c.notifyListeners()
		}
		if c.direction == animationDirectionForward {
			c.status = AnimationStatusCompleted
		} else {
			c.status = AnimationStatusDismissed
		}
		c.checkStatusChanged()
		return
	}
	debug.Assert(simulationDuration > 0)
	debug.Assert(!c.Animating())
	c.startSimulation(&interpolationSimulation{
		Begin:     c.value,
		End:       v,
		Duration:  simulationDuration,
		Curve:     curve,
		Tolerance: DefaultTolerance,
	})
}

func (c *Controller) Repeat(reverse bool, count int) {
	// TODO(dh): support curve on repeating animation
	c.Stop()
	sim := newRepeatingSimulation(
		c.value,
		c.LowerBound,
		c.UpperBound,
		reverse,
		c.Duration,
		count,
		c.setDirection,
	)
	c.startSimulation(sim)
}

func (c *Controller) Stop() {
	c.simulation = nil
	c.lastElapsedDuration = 0
	c.ticker.Stop()
}

func (c *Controller) startSimulation(sim Simulation) {
	debug.Assert(!c.Animating())
	c.simulation = sim
	c.lastElapsedDuration = 0
	c.value = jmath.Clamp(sim.X(0), c.LowerBound, c.UpperBound)
	c.ticker.Start()
	if c.direction == animationDirectionForward {
		c.status = AnimationStatusForward
	} else {
		c.status = AnimationStatusReverse
	}
	c.checkStatusChanged()
}

func (c *Controller) AddListener(cb func()) base.Listener {
	return c.listeners.AddListener(cb)
}

func (c *Controller) RemoveListener(l base.Listener) {
	c.listeners.RemoveListener(l)
}

func (c *Controller) ClearListeners() {
	c.listeners.ClearListeners()
}

func (c *Controller) notifyListeners() {
	c.listeners.NotifyListeners()
}

func (c *Controller) AddStatusListener(cb func(status AnimationStatus)) StatusListener {
	return c.statusListeners.AddStatusListener(cb)

}
func (c *Controller) RemoveStatusListener(l StatusListener) {
	c.statusListeners.RemoveStatusListener(l)
}

func (c *Controller) ClearStatusListeners() {
	c.statusListeners.ClearStatusListeners()
}

func (c *Controller) notifyStatusListeners(status AnimationStatus) {
	c.statusListeners.NotifyStatusListeners(status)
}

func (c *Controller) tick(elapsed time.Duration) {
	c.lastElapsedDuration = elapsed
	c.value = jmath.Clamp(c.simulation.X(elapsed), c.LowerBound, c.UpperBound)
	if c.simulation.Done(elapsed) {
		if c.direction == animationDirectionForward {
			c.status = AnimationStatusCompleted
		} else {
			c.status = AnimationStatusDismissed
		}
		c.Stop()
	}
	c.notifyListeners()
	c.checkStatusChanged()
}

func (c *Controller) checkStatusChanged() {
	if c.lastReportedStatus != c.status {
		c.lastReportedStatus = c.status
		c.notifyStatusListeners(c.status)
	}
}

func (c *Controller) setDirection(dir animationDirection) {
	c.direction = dir
	if dir == animationDirectionForward {
		c.status = AnimationStatusForward
	} else {
		c.status = AnimationStatusReverse
	}
	c.checkStatusChanged()
}

func (c *Controller) Dispose() {
	c.ticker.Dispose()
	c.ClearListeners()
	c.ClearStatusListeners()
}
