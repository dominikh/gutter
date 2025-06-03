// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package animation

import (
	"math"
	"time"

	"honnef.co/go/gutter/gmath"
)

type Tolerance struct {
	Distance float64
	Time     time.Duration
	Velocity float64
}

var DefaultTolerance = Tolerance{
	Distance: 0.001,
	Time:     time.Nanosecond,
	Velocity: 0.001,
}

type Simulation interface {
	Dx(d time.Duration) float64
	X(d time.Duration) float64
	Done(d time.Duration) bool
}

var _ Simulation = (*interpolationSimulation)(nil)

type interpolationSimulation struct {
	Duration  time.Duration
	Begin     float64
	End       float64
	Curve     Curve
	Tolerance Tolerance
}

func (sim *interpolationSimulation) X(d time.Duration) float64 {
	t := gmath.Clamp(float64(d)/float64(sim.Duration), 0, 1)
	switch t {
	case 0:
		return sim.Begin
	case 1:
		return sim.End
	default:
		return sim.Begin + (sim.End-sim.Begin)*sim.Curve.Transform(t)
	}
}

func (sim *interpolationSimulation) Dx(d time.Duration) float64 {
	ϵ := sim.Tolerance.Time
	return (sim.X(d+ϵ) - sim.X(d-ϵ)) / float64(2*ϵ)
}

func (sim *interpolationSimulation) Done(d time.Duration) bool {
	return d > sim.Duration
}

var _ Simulation = (*repeatingSimulation)(nil)

type directionSetter func(dir animationDirection)

type repeatingSimulation struct {
	Min             float64
	Max             float64
	Reverse         bool
	Count           int
	DirectionSetter directionSetter
	Period          time.Duration

	initialTime float64
	exitTime    float64
}

func newRepeatingSimulation(
	initial, min, max float64,
	reverse bool,
	period time.Duration,
	count int,
	ds directionSetter,
) *repeatingSimulation {
	var initialTime float64
	if min != max {
		initialTime = ((gmath.Clamp(initial, min, max) - min) / (max - min)) * period.Seconds()
	}
	var exitTime float64
	if count > 0 {
		exitTime = (float64(count) * period.Seconds()) - initialTime
	}

	return &repeatingSimulation{
		Min:             min,
		Max:             max,
		Reverse:         reverse,
		Period:          period,
		Count:           count,
		DirectionSetter: ds,
		initialTime:     initialTime,
		exitTime:        exitTime,
	}
}

func (sim *repeatingSimulation) X(d time.Duration) float64 {
	totalTime := d.Seconds() + sim.initialTime
	_, t := math.Modf(totalTime / float64(sim.Period.Seconds()))
	isPlayingReverse := uint64(totalTime/sim.Period.Seconds())%2 != 0

	if sim.Reverse && isPlayingReverse {
		sim.DirectionSetter(animationDirectionReverse)
		return Lerp(sim.Max, sim.Min, t)
	} else {
		sim.DirectionSetter(animationDirectionForward)
		return Lerp(sim.Min, sim.Max, t)
	}
}

func (sim *repeatingSimulation) Dx(d time.Duration) float64 {
	return (sim.Max - sim.Min) / sim.Period.Seconds()
}

func (sim *repeatingSimulation) Done(d time.Duration) bool {
	return sim.Count > 0 && d.Seconds() >= sim.exitTime
}
