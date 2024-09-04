// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"math/rand"
	"time"

	"honnef.co/go/color"
	"honnef.co/go/gutter/application"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
)

func main() {
	log.SetFlags(log.Lmicroseconds)

	theCh := make(chan color.Color)
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			r := rand.Float64()
			g := rand.Float64()
			b := rand.Float64()

			c := color.Make(color.LinearSRGB, r, g, b, 1)
			theCh <- c
		}
	}()

	root := &widget.Flex{
		Direction:          render.Horizontal,
		MainAxisAlignment:  render.MainAxisAlignCenter,
		CrossAxisAlignment: render.CrossAxisAlignCenter,
		MainAxisSize:       render.MainAxisSizeMax,
		Children: []widget.Widget{
			&widget.Flexible{
				Fit: render.FlexFitTight,
				Child: &widget.Builder{
					Builder: func(ctx widget.BuildContext, child widget.Widget) widget.Widget {
						log.Println("building sizey boy")
						width := widget.DependOnWidgetOfExactType[*widget.MediaQuery](ctx).Data.Size.Width
						return &widget.SizedBox{
							Width:  width / 4,
							Height: 200,
							Child: &widget.ColoredBox{
								Color: color.Make(color.LinearSRGB, 0, 1, 0, 1),
							},
						}
					},
				},
			},

			&widget.Flexible{
				Fit: render.FlexFitTight,
				Child: &widget.SizedBox{
					Width:  200,
					Height: 200,
					Child: &widget.ChannelBuilder[color.Color]{
						Channel: theCh,
						Builder: func(ctx widget.BuildContext, child widget.Widget, v color.Color) widget.Widget {
							log.Println("building colory boy")
							if v == (color.Color{}) {
								v = color.Make(color.LinearSRGB, 1, 0, 1, 1)
							}
							return &widget.ColoredBox{
								Color: v,
							}
						},
					},
				},
			},
		},
	}

	application.Run(context.Background(), root)
}

type ColorChangingBox struct {
	Color color.Color
}

func (c *ColorChangingBox) CreateElement() widget.Element {
	return widget.NewInteriorElement(c)
}

func (c *ColorChangingBox) CreateState() widget.State[*ColorChangingBox] {
	return &colorChangingBoxState{c: c.Color}
}

type colorChangingBoxState struct {
	widget.StateHandle[*ColorChangingBox]
	c color.Color
}

func (cs *colorChangingBoxState) Transition(t widget.StateTransition[*ColorChangingBox]) {
}

func (cs *colorChangingBoxState) Build(ctx widget.BuildContext) widget.Widget {
	return &widget.PointerRegion{
		OnPress: func(hit render.HitTestEntry, ev pointer.Event) {
			// cs.c.R += 50
			widget.MarkNeedsBuild(cs.Element)
		},
		Child: &widget.SizedBox{
			Width: 100,
			Child: &widget.ColoredBox{
				Color: cs.c,
			},
		},
	}
}
