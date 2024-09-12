// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"honnef.co/go/color"
	"honnef.co/go/gutter/application"
	"honnef.co/go/gutter/lottie/lottie_converter"
	"honnef.co/go/gutter/lottie/lottie_encoding"
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/gutter/wsi"
)

func main() {
	log.SetFlags(log.Lmicroseconds | log.Llongfile)
	// slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	// 	AddSource: true,
	// 	Level:     slog.LevelDebug,
	// })))

	app, err := application.New()
	if err != nil {
		log.Fatal(err)
	}
	defer app.Dispose()

	theCh := make(chan float64)
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			theCh <- rand.Float64()
		}
	}()

	b, err := os.ReadFile("/home/dominikh/lottie/:melting:.json")
	if err != nil {
		panic(err)
	}
	anim, err := lottie_encoding.Parse(b)
	if err != nil {
		panic(err)
	}
	comp := lottie_converter.ConvertAnimation(anim)

	l := widget.NewChannelListener(theCh, func(ev wsi.Event) {
		// Ignore that we need this argument, ideally widget.NewChannelListener
		// would have a magic way to get the event emitter, while not depending
		// on the 'application' package.
		app.EmitEvent(nil, ev)
	})
	defer l.Dispose()

	root := &widget.ValueListenableBuilder[float64]{
		ValueListenable: l,
		Builder: func(ctx widget.BuildContext, mv maybe.Option[float64], child widget.Widget) widget.Widget {
			return &widget.Column{
				Children: []widget.Widget{
					&widget.Lottie{
						Composition: comp,
						Animate:     true,
						Repeat:      true,
						Reverse:     false,
						Width:       64,
					},
					&widget.Flexible{
						Flex: 1,
						Fit:  render.FlexFitTight,
						Child: &widget.ColoredBox{
							Color: color.Make(color.SRGB, 1, 0, 0, 1),
						},
					},
					&widget.Lottie{
						Composition: comp,
						Animate:     true,
						Repeat:      true,
						Reverse:     false,
						Width:       64,
					},
				},
			}
		},
	}

	// root := &widget.Lottie{
	// 	Composition: comp,
	// 	Animate:     true,
	// 	Repeat:      true,
	// 	Reverse:     false,
	// }

	app.Run(context.Background(), root)
}
