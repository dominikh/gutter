// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package application

import (
	"context"
	"fmt"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
	"honnef.co/go/jello/engine/wgpu_engine"
	"honnef.co/go/jello/mem"
	"honnef.co/go/jello/renderer"
	"honnef.co/go/wgpu"
)

func Run(ctx context.Context, root widget.Widget) error {
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
		return err
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
		return err
	}

	queue := dev.Queue()
	defer queue.Release()

	r := wgpu_engine.New(dev, &wgpu_engine.RendererOptions{
		SurfaceFormat: wgpu.TextureFormatRGBA8UnormSrgb,
		UseCPU:        false,
	})

	app := &application{
		root:   root,
		dev:    dev,
		ins:    ins,
		engine: r,
		queue:  queue,
	}
	app.sys = wsi.NewSystem(app)
	return app.sys.Run(context.Background())
}

type application struct {
	root          widget.Widget
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
func (a *application) UserEvent(*wsi.Context, any) {
	panic("unimplemented")
}

// WindowEvent implements wsi.Application.
func (app *application) WindowEvent(ctx *wsi.Context, ev wsi.Event) {
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

		app.widgetBinding = widget.RunApp(app.sys, app.win, app.root)

	case *wsi.Resized:
		if ev.Size == (wsi.LogicalSize{}) {
			ev.Size = wsi.LogicalSize{Width: 500, Height: 500}
		}
		sz := curve.Sz(float64(ev.Size.Width), float64(ev.Size.Height))
		app.widgetBinding.PipelineOwner.View().SetConfiguration(render.ViewConfiguration{
			Min: sz,
			Max: sz,
		})
		app.size = ev.Size
		app.resized = true
		app.win.SetSize(ev.Size)
		app.win.SetScale(1)

	case *wsi.RedrawRequested:
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

		app.widgetBinding.DrawFrame(ev, &app.scene)

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
				BaseColor:          color.Make(color.LinearSRGB, 1, 1, 1, 1),
				Width:              uint32(app.size.Width),
				Height:             uint32(app.size.Height),
				AntialiasingMethod: renderer.Area,
			},
			nil,
		)

		app.surface.Present()

	case widget.CallbackEvent:
		ev()

	default:
		panic(fmt.Sprintf("internal error: unhandled event type %T", ev))
	}
}
