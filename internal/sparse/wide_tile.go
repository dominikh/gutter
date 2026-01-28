// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"

	"honnef.co/go/gutter/gfx"
)

const disableWideTileOpts = false
const wideTileWidth = 256

type wideTile struct {
	bg             gfx.PlainColor
	cmds           []cmd
	fillArgs       []fillArgs
	alphaFillArgs  []alphaFillArgs
	blendArgs      []blendArgs
	alphaBlendArgs []alphaBlendArgs
	numZeroClips   int
	numLayers      int
}

// TODO rename to cmdKind, same for cmd.typ field
//
//go:generate go tool stringer -type=cmdType
type cmdType uint8

const (
	cmdNop cmdType = iota
	cmdFill
	cmdAlphaFill
	cmdPushLayer
	cmdPopLayer
	cmdCopyBackdrop
	cmdBlend
	cmdAlphaBlend
	// cmdClear sets all pixels to the specified color
	//
	// TODO(dh): we won't need this once cmdFill supports blend modes
	cmdClear
)

type baseArgs struct {
	x     uint16
	width uint16
}

type fillArgs struct {
	paint encodedPaint
	baseArgs
}

type alphaFillArgs struct {
	alphas [][stripHeight]uint8
	fillArgs
}

type blendArgs struct {
	opacity float32
	baseArgs
	blend gfx.BlendMode
}

type alphaBlendArgs struct {
	alphas [][stripHeight]uint8
	blendArgs
}

type cmd struct {
	typ  cmdType
	args uint32
}

func (wt *wideTile) stringifyCmd(cmd cmd) string {
	switch cmd.typ {
	case cmdFill:
		args := wt.fillArgs[cmd.args]
		return fmt.Sprintf("Fill(x=%v, width=%v, paint=%v)",
			args.x, args.width, args.paint)
	case cmdAlphaFill:
		args := wt.alphaFillArgs[cmd.args]
		return fmt.Sprintf("AlphaFill(x=%v, width=%v, paint=%v)",
			args.x, args.width, args.paint)
	case cmdPushLayer:
		return "PushLayer()"
	case cmdPopLayer:
		return "PopLayer()"
	case cmdCopyBackdrop:
		return "CopyBackdrop()"
	case cmdBlend:
		args := wt.blendArgs[cmd.args]
		return fmt.Sprintf("Blend(x=%v, width=%v, blend=%s, opacity=%v)",
			args.x, args.width, args.blend, args.opacity)
	case cmdAlphaBlend:
		args := wt.alphaBlendArgs[cmd.args]
		return fmt.Sprintf("AlphaBlend(x=%v, width=%v, blend=%s, opacity=%v)",
			args.x, args.width, args.blend, args.opacity)
	case cmdNop:
		return "Nop()"
	case cmdClear:
		args := wt.fillArgs[cmd.args]
		return fmt.Sprintf("Clear(x=%v, width=%v, paint=%v)",
			args.x, args.width, args.paint)
	default:
		panic(fmt.Sprintf("invalid command type %v", cmd.typ))
	}
}

// func (cmd cmd) String() string {
// }

func (wt *wideTile) fill(x, width uint16, paint encodedPaint) {
	if wt.isZeroClip() {
		return
	}
	t := cmdFill
	if paint.Opaque() {
		t = cmdClear
	}
	wt.fillArgs = append(wt.fillArgs, fillArgs{
		paint: paint,
		baseArgs: baseArgs{
			x:     x,
			width: width,
		},
	})
	wt.cmds = append(wt.cmds, cmd{typ: t, args: uint32(len(wt.fillArgs) - 1)})
}

func (wt *wideTile) alphaFill(args alphaFillArgs) {
	if wt.isZeroClip() {
		return
	}
	wt.alphaFillArgs = append(wt.alphaFillArgs, args)
	wt.cmds = append(wt.cmds, cmd{typ: cmdAlphaFill, args: uint32(len(wt.alphaFillArgs) - 1)})
}

func (wt *wideTile) pushLayer() {
	if wt.isZeroClip() {
		return
	}

	wt.cmds = append(wt.cmds, cmd{typ: cmdPushLayer})
	wt.numLayers++
}

func (wt *wideTile) copyBackdrop() {
	if wt.isZeroClip() {
		return
	}

	wt.cmds = append(wt.cmds, cmd{typ: cmdCopyBackdrop})
}

func (wt *wideTile) popLayer() {
	if wt.isZeroClip() {
		return
	}
	if !disableWideTileOpts &&
		len(wt.cmds) > 0 &&
		wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer {
		// Nothing was drawn inside the layer, elide it.
		wt.cmds = wt.cmds[:len(wt.cmds)-1]
	} else {
		wt.cmds = append(wt.cmds, cmd{typ: cmdPopLayer})
	}
	wt.numLayers--
}

func (wt *wideTile) pushZeroClip() {
	wt.numZeroClips++
}

func (wt *wideTile) popZeroClip() {
	if wt.numZeroClips == 0 {
		panic("internal error: unbalanced zero clips")
	}
	wt.numZeroClips--
}

func (wt *wideTile) isZeroClip() bool {
	return wt.numZeroClips > 0
}

func (wt *wideTile) alphaBlend(args alphaBlendArgs) {
	if wt.isZeroClip() {
		return
	}
	if !disableWideTileOpts &&
		len(wt.cmds) > 0 &&
		wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer &&
		args.blend.Compose&gfx.ComposeAffectsDestRegion == 0 {
		return
	}
	wt.alphaBlendArgs = append(wt.alphaBlendArgs, args)
	wt.cmds = append(wt.cmds, cmd{typ: cmdAlphaBlend, args: uint32(len(wt.alphaBlendArgs) - 1)})
}

func (wt *wideTile) blend(
	x uint16,
	width uint16,
	blend gfx.BlendMode,
	opacity float32,
) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) == 0 {
		panic("internal error: called blend without pushing a layer")
	}
	prevCmd := &wt.cmds[len(wt.cmds)-1]
	if !disableWideTileOpts &&
		blend.Compose&gfx.ComposeAffectsDestRegion == 0 &&
		prevCmd.typ == cmdPushLayer {
		// Blending when nothing has been drawn in the layer yet has no visible
		// effect for some compose operators, notably SrcOver.
		return
	}

	// We don't check that the blend mode and opacity match, because at command
	// generation time, an uninterrupted run of blends is only possible while
	// popping a layer.
	if prevCmd.typ == cmdBlend {
		prevArgs := wt.blendArgs[prevCmd.args]
		if !disableWideTileOpts && x == prevArgs.x+prevArgs.width {
			prevArgs.width += width
			return
		}
	}
	wt.blendArgs = append(wt.blendArgs, blendArgs{
		blend:   blend,
		opacity: opacity,
		baseArgs: baseArgs{
			x:     x,
			width: width,
		},
	})
	wt.cmds = append(wt.cmds, cmd{typ: cmdBlend, args: uint32(len(wt.blendArgs) - 1)})
}

func (wt *wideTile) baseArgs(c cmd) *baseArgs {
	switch c.typ {
	case cmdFill:
		return &wt.fillArgs[c.args].baseArgs
	case cmdAlphaFill:
		return &wt.alphaFillArgs[c.args].baseArgs
	case cmdClear:
		return &wt.fillArgs[c.args].baseArgs
	case cmdBlend:
		return &wt.blendArgs[c.args].baseArgs
	case cmdAlphaBlend:
		return &wt.alphaBlendArgs[c.args].baseArgs
	case cmdCopyBackdrop:
		return nil
	case cmdNop:
		return nil
	case cmdPopLayer:
		return nil
	case cmdPushLayer:
		return nil
	default:
		panic(fmt.Sprintf("unexpected sparse.cmdType: %#v", c.typ))
	}
}

func (wt *wideTile) getBlendArgs(c cmd) *blendArgs {
	switch c.typ {
	case cmdBlend:
		return &wt.blendArgs[c.args]
	case cmdAlphaBlend:
		return &wt.alphaBlendArgs[c.args].blendArgs
	default:
		return nil
	}
}
