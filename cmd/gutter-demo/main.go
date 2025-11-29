// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"honnef.co/go/color"
	"honnef.co/go/gutter/application"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/gutter/widget/widgets"
	"honnef.co/go/stuff/container/maybe"
)

var _ widget.StatefulWidget[*Root] = (*Root)(nil)
var _ widget.StatelessWidget = (*Interior1)(nil)
var _ widget.StatelessWidget = (*Interior2)(nil)
var _ widget.StatelessWidget = (*Leaf)(nil)

type Root struct {
	Child widget.Widget
}

// CreateElement implements widget.StatelessWidget.
func (r *Root) CreateElement() widget.Element {
	return widget.NewInteriorElement(r)
}

// CreateState implements widget.StatefulWidget.
func (r *Root) CreateState() widget.State[*Root] {
	return &rootState{}
}

var int2 = &Interior2{
	Child: &Leaf{},
}

var ch = make(chan float64)

type rootState struct {
	widget.StateHandle[*Root]

	l *widgets.ChannelListener[float64]
}

// Build implements widget.State.
func (r *rootState) Build(ctx widget.BuildContext) widget.Widget {
	log.Println("Building Root")
	return &widgets.ValueListenableBuilder[float64]{
		ValueListenable: r.l,
		Builder: func(ctx widget.BuildContext, v maybe.Option[float64], child widget.Widget) widget.Widget {
			log.Println("building")
			return &Interior1{
				R:     v.UnwrapOr(1),
				Child: int2,
			}
		},
	}
}

// Transition implements widget.State.
func (r *rootState) Transition(t widget.StateTransition[*Root]) {
	log.Println(t)
	switch t.Kind {
	case widget.StateInitializing:
		r.l = widgets.NewChannelListener(ch, r.BuildOwner().EmitEvent)
	case widget.StateUpdatedWidget:
	case widget.StateChangedDependencies:
	case widget.StateActivating:
	case widget.StateDeactivating:
	case widget.StateDisposing:
		r.l.Dispose()
	default:
		panic(fmt.Sprintf("unexpected widget.StateTransitionKind: %#v", t.Kind))
	}
}

type Interior1 struct {
	R     float64
	Child widget.Widget
}

// Build implements widget.StatelessWidget.
func (i *Interior1) Build(ctx widget.BuildContext) widget.Widget {
	log.Println("Building Interior1")
	return i.Child
}

type Interior2 struct {
	Child widget.Widget
}

// Build implements widget.StatelessWidget.
func (i *Interior2) Build(ctx widget.BuildContext) widget.Widget {
	log.Println("Building Interior2")
	return i.Child
}

type Leaf struct{}

// Build implements widget.StatelessWidget.
func (l *Leaf) Build(ctx widget.BuildContext) widget.Widget {
	log.Printf("Building Leaf %p", l)
	int1 := widget.Ancestor[*Interior1](ctx)
	return &widgets.ColoredBox{
		Color: color.Make(color.SRGB, int1.R, 0, 0, 1),
	}
}

func main() {
	log.SetFlags(log.Lmicroseconds | log.Llongfile)

	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			ch <- rand.Float64()
		}
	}()

	app, err := application.New()
	if err != nil {
		log.Fatal(err)
	}
	// XXX
	// defer app.Dispose()

	root := &Root{
		Child: &Interior1{
			Child: &Interior2{
				Child: &Leaf{},
			},
		},
	}
	app.Run(context.Background(), root)
}
