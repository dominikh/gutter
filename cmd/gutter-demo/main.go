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
	"honnef.co/go/curve"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
	"honnef.co/go/jello/engine/wgpu_engine"
	"honnef.co/go/jello/mem"
	"honnef.co/go/jello/renderer"
	"honnef.co/go/wgpu"
)

type Application struct {
	dev           *wgpu.Device
	ins           *wgpu.Instance
	engine        *wgpu_engine.Engine
	queue         *wgpu.Queue
	sys           *wsi.System
	win           *wsi.WaylandWindow
	surface       *wgpu.Surface
	arena         mem.Arena
	scene         jello.Scene
	widgetBinding *widget.Binding
	resized       bool
	size          wsi.LogicalSize
}

// UserEvent implements wsi.Application.
func (a *Application) UserEvent(*wsi.Context, any) {
	panic("unimplemented")
}

var theCh = make(chan color.Color)

// WindowEvent implements wsi.Application.
func (app *Application) WindowEvent(ctx *wsi.Context, ev wsi.Event) {
	switch ev := ev.(type) {
	case *wsi.EventInitialized:
		app.win = ctx.CreateWindow().(*wsi.WaylandWindow)
		app.surface = app.ins.CreateSurface(wgpu.SurfaceDescriptor{
			Label: "our surface",
			Native: wgpu.WaylandSurface{
				Display: app.win.Display(),
				Surface: app.win.Surface(),
			},
		})

		root := &widget.Builder{
			Builder: func(ctx widget.BuildContext, _ widget.Widget) widget.Widget {
				// _ = widget.DependOnWidgetOfExactType[*widget.MediaQuery](ctx).Data.Size.Width
				now := rand.Int()
				return &widget.Flex{
					Direction:          render.Horizontal,
					MainAxisAlignment:  render.MainAxisAlignStart,
					CrossAxisAlignment: render.CrossAxisAlignCenter,
					MainAxisSize:       render.MainAxisSizeMax,
					Children: []widget.Widget{
						&widget.Flexible{
							Fit: render.FlexFitTight,
							Child: &widget.SizedBox{
								Width:  200,
								Height: 200,
								Child: &widget.ColoredBox{
									Color: color.Make(color.LinearSRGB, 0, float64(uint8(now))/255.0, 0, 1),
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
										log.Println("building")
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
			},
		}

		app.widgetBinding = widget.RunApp(app.sys, app.win, root)

	case *wsi.Resized:
		if ev.Size == (wsi.LogicalSize{}) {
			ev.Size = wsi.LogicalSize{Width: 500, Height: 500}
		}
		sz := curve.Sz(float64(ev.Size.Width), float64(ev.Size.Height))
		app.widgetBinding.RenderBinding.View().SetConfiguration(render.ViewConfiguration{
			Min: sz,
			Max: sz,
		})
		app.size = ev.Size
		app.resized = true
		app.win.SetSize(ev.Size)
		app.win.SetScale(1)

	case *wsi.RedrawRequested:
		log.Println("frame")
		if app.resized {
			app.resized = false
			app.surface.Configure(app.dev, &wgpu.SurfaceConfiguration{
				Width:                      uint32(app.size.Width),
				Height:                     uint32(app.size.Height),
				Format:                     wgpu.TextureFormatRGBA8UnormSrgb,
				Usage:                      wgpu.TextureUsageRenderAttachment,
				PresentMode:                wgpu.PresentModeMailbox,
				AlphaMode:                  wgpu.CompositeAlphaModeAuto,
				DesiredMaximumFrameLatency: 2,
			})
		}

		app.arena.Reset()
		app.scene.Reset()

		app.widgetBinding.DrawFrame(&app.scene)

		surfaceTex, err := app.surface.CurrentTexture()
		if err != nil {
			panic(err)
		}
		defer surfaceTex.Texture.Release()

		// XXX DPI scale

		app.engine.RenderToSurface(
			&app.arena,
			app.queue,
			app.scene.Encoding(),
			&surfaceTex,
			&renderer.RenderParams{
				BaseColor:          color.Make(color.LinearSRGB, 1, 0, 0, 1),
				Width:              uint32(app.size.Width),
				Height:             uint32(app.size.Height),
				AntialiasingMethod: renderer.Area,
			},
			nil,
		)

		app.surface.Present()

	case widget.CallbackEvent:
		log.Println("running callback")
		ev()

	default:
		panic(fmt.Sprintf("internal error: unhandled event type %T", ev))
	}
}

func main() {
	log.SetFlags(log.Lmicroseconds)

	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			r := rand.Float64()
			g := rand.Float64()
			b := rand.Float64()

			c := color.Make(color.LinearSRGB, r, g, b, 1)
			theCh <- c
			log.Println("sent color")
		}
	}()

	ins := wgpu.CreateInstance(wgpu.InstanceDescriptor{
		Extras: &wgpu.InstanceExtras{
			Backends: wgpu.InstanceBackendVulkan,
		},
	})
	defer ins.Release()

	adapter, err := ins.RequestAdapter(wgpu.RequestAdapterOptions{
		PowerPreference:      wgpu.PowerPreferenceHighPerformance,
		ForceFallbackAdapter: false,
		// CompatibleSurface:    surface,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer adapter.Release()

	supportedLimits := adapter.Limits()

	limits := wgpu.DefaultLimits
	limits.MaxSampledTexturesPerShaderStage = 8388606
	limits.MaxTextureDimension1D = supportedLimits.MaxTextureDimension1D
	limits.MaxTextureDimension2D = supportedLimits.MaxTextureDimension2D
	limits.MaxTextureDimension3D = supportedLimits.MaxTextureDimension3D
	dev, err := adapter.RequestDevice(&wgpu.DeviceDescriptor{
		RequiredFeatures: []wgpu.FeatureName{
			wgpu.FeatureNameTimestampQuery,
			wgpu.NativeFeatureNameTextureBindingArray,
			wgpu.NativeFeatureNameSampledTextureAndStorageBufferArrayNonUniformIndexing,
			wgpu.NativeFeatureNamePartiallyBoundBindingArray,
		},
		RequiredLimits: &wgpu.RequiredLimits{
			Limits: limits,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	queue := dev.Queue()
	defer queue.Release()

	r := wgpu_engine.New(dev, &wgpu_engine.RendererOptions{
		SurfaceFormat: wgpu.TextureFormatRGBA8UnormSrgb,
		UseCPU:        false,
	})

	app := &Application{
		dev:    dev,
		ins:    ins,
		engine: r,
		queue:  queue,
	}
	app.sys = wsi.NewSystem(app)
	app.sys.Run(context.Background())
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
