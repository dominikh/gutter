// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"honnef.co/go/color"
	"honnef.co/go/gutter/application"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/lottie/lottie_converter"
	"honnef.co/go/gutter/lottie/lottie_encoding"
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

	paths, _ := filepath.Glob("/home/dominikh/lottie/*.json")
	rand.Shuffle(len(paths), func(i, j int) {
		paths[i], paths[j] = paths[j], paths[i]
	})
	var rows []widget.Widget
	var row *widget.Flex
	for i, path := range paths[:100] {
		if i%10 == 0 {
			row = &widget.Flex{
				Direction:          render.Horizontal,
				MainAxisAlignment:  render.MainAxisAlignCenter,
				CrossAxisAlignment: render.CrossAxisAlignCenter,
				MainAxisSize:       render.MainAxisSizeMax,
			}
			rows = append(rows, row)
		}

		b, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		anim, err := lottie_encoding.Parse(b)
		if err != nil {
			panic(err)
		}
		comp := lottie_converter.ConvertAnimation(anim)

		w := &widget.Flexible{
			Fit: render.FlexFitTight,
			Child: &widget.Lottie{
				Composition: comp,
				Width:       64.0,
			},
		}
		row.Children = append(row.Children, w)
	}

	root := &widget.Flex{
		Direction:          render.Vertical,
		MainAxisAlignment:  render.MainAxisAlignCenter,
		CrossAxisAlignment: render.CrossAxisAlignCenter,
		MainAxisSize:       render.MainAxisSizeMax,
		Children:           rows,
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
