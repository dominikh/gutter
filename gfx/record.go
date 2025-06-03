// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gfx

import (
	"fmt"

	"honnef.co/go/curve"
)

type Recording []Command

type Command interface {
	isCommand()
}

// OPT(dh): we could save space by encoding fill rule and other changes as
// commands. However, that would make it more difficult to nest recordings,
// requiring saving and restoring all state using additional commands.

type CommandPushLayer struct {
	Layer     Layer
	FillRule  FillRule
	Transform curve.Affine
}

type CommandPopLayer struct{}

type CommandFill struct {
	Shape     Shape
	Paint     Paint
	Transform curve.Affine
	FillRule  FillRule
}

type CommandStroke struct {
	Shape     Shape
	Paint     Paint
	Stroke    curve.Stroke
	Transform curve.Affine
}

type CommandPlayRecording struct {
	Recording Recording
	Transform curve.Affine
}

func (cmd CommandFill) GoString() string {
	return fmt.Sprintf("gfx.CommandFill{Shape: %#v, Paint: %#v, Transform: %#v, FillRule: %d}",
		cmd.Shape, cmd.Paint, cmd.Transform, cmd.FillRule)
}

func (CommandPushLayer) isCommand()     {}
func (CommandPopLayer) isCommand()      {}
func (CommandFill) isCommand()          {}
func (CommandStroke) isCommand()        {}
func (CommandPlayRecording) isCommand() {}

type Recorder interface {
	PushTransform(curve.Affine)
	CurrentTransform() curve.Affine
	PopTransform()

	SetFillRule(FillRule)

	PushClip(Shape)
	PushLayer(Layer)
	PopLayer()

	Fill(Shape, Paint)
	Stroke(Shape, curve.Stroke, Paint)
	// TODO docs
	//
	// PlayRecording does not take the current transformation into
	// consideration. All commands in a recording use absolute coordinates.
	PlayRecording(Recording)

	// Checkpoint returns a new recorder that copies this recorder's current
	// state, but whose layers and transforms cannot be popped beyond their
	// current state. The old and new recorder share the same list of recorded
	// commands and commands added via one recorder are immediately visible to
	// the other.
	Checkpoint() Recorder
	Finish() Recording
}

func NewSimpleRecorder() *SimpleRecorder {
	return &SimpleRecorder{
		transform: curve.Identity,
		commands:  new([]Command),
	}
}

type SimpleRecorder struct {
	transform      curve.Affine
	transformStack []curve.Affine
	fillRule       FillRule

	layerCount int
	commands   *[]Command
}

// Fill implements Recorder.
func (s *SimpleRecorder) Fill(shape Shape, paint Paint) {
	*s.commands = append(*s.commands, CommandFill{
		Paint:     paint,
		Shape:     shape,
		Transform: s.transform,
		FillRule:  s.fillRule,
	})
}

// Stroke implements Recorder.
func (s *SimpleRecorder) Stroke(shape Shape, stroke curve.Stroke, paint Paint) {
	*s.commands = append(*s.commands, CommandStroke{
		Shape:     shape,
		Paint:     paint,
		Stroke:    stroke,
		Transform: s.transform,
	})
}

// PlayRecording implements Recorder.
func (s *SimpleRecorder) PlayRecording(rec Recording) {
	*s.commands = append(*s.commands, CommandPlayRecording{
		Recording: rec,
		Transform: s.transform,
	})
}

// PushClip implements Recorder.
func (s *SimpleRecorder) PushClip(shape Shape) {
	s.PushLayer(Layer{
		Opacity: 1,
		Clip:    shape,
	})
}

// PushLayer implements Recorder.
func (s *SimpleRecorder) PushLayer(l Layer) {
	*s.commands = append(*s.commands, CommandPushLayer{
		Layer:     l,
		FillRule:  s.fillRule,
		Transform: s.transform,
	})
	s.layerCount++
}

// PopLayer implements Recorder.
func (s *SimpleRecorder) PopLayer() {
	if s.layerCount <= 0 {
		panic("unbalanced layer push/pop")
	}
	*s.commands = append(*s.commands, CommandPopLayer{})
	s.layerCount--
}

// PushTransform implements Recorder.
func (s *SimpleRecorder) PushTransform(aff curve.Affine) {
	s.transformStack = append(s.transformStack, s.transform)
	s.transform = s.transform.Mul(aff)
}

func (s *SimpleRecorder) CurrentTransform() curve.Affine {
	return s.transform
}

// PopTransform implements Recorder.
func (s *SimpleRecorder) PopTransform() {
	if len(s.transformStack) == 0 {
		panic("unbalanced transform push/pop")
	}
	s.transform = s.transformStack[len(s.transformStack)-1]
	s.transformStack = s.transformStack[:len(s.transformStack)-1]
}

// SetFillRule implements Recorder.
func (s *SimpleRecorder) SetFillRule(fr FillRule) {
	s.fillRule = fr
}

// Checkpoint implements Recorder.
func (s *SimpleRecorder) Checkpoint() Recorder {
	return &SimpleRecorder{
		transform:      s.transform,
		transformStack: nil,
		fillRule:       s.fillRule,
		layerCount:     0,
		commands:       s.commands,
	}
}

// Finish implements Recorder.
func (s *SimpleRecorder) Finish() Recording {
	for s.layerCount > 0 {
		s.PopLayer()
	}
	return Recording(*s.commands)
}
