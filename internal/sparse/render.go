// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"image"
	"slices"

	"honnef.co/go/curve"
)

// use std::collections::BTreeMap;

// use piet_next::{
//     peniko::{
//         color::{palette, AlphaColor, Srgb},
//         kurbo::Affine,
//         BrushRef,
//     },
//     GenericRecorder, RenderCtx, ResourceCtx,
// };

// use crate::{
//     fine::Fine,
//     strip::{self, Strip, Tile},
//     tiling::{self, FlatLine},
//     wide_tile::{Cmd, CmdStrip, WideTile, STRIP_HEIGHT, WIDE_TILE_WIDTH},
//     Pixmap,
// };

type CsRenderCtx struct {
	width  int
	height int
	tiles  []wideTile
	alphas []uint32

	/// These are all scratch buffers, to be used for path rendering. They're here solely
	/// so the allocations can be reused.
	line_buf  []flatLine
	tile_buf  []tile
	strip_buf []strip

	transform curve.Affine
}

// pub struct CsResourceCtx;

func NewCsRenderCtx(width, height int) *CsRenderCtx {
	width_tiles := (width + WIDE_TILE_WIDTH - 1) / WIDE_TILE_WIDTH
	height_tiles := (height + STRIP_HEIGHT - 1) / STRIP_HEIGHT
	tiles := make([]wideTile, width_tiles*height_tiles)

	return &CsRenderCtx{
		width:     width,
		height:    height,
		tiles:     tiles,
		transform: curve.Identity,
	}
}

func (ctx *CsRenderCtx) Reset() {
	for i := range ctx.tiles {
		tile := &ctx.tiles[i]
		tile.bg = [4]float32{}
		clear(tile.cmds)
		tile.cmds = tile.cmds[:0]
	}
}

func (ctx *CsRenderCtx) RenderToPixmap(pixmap *image.RGBA) {
	fine := fine{
		width:   pixmap.Bounds().Dx(),
		height:  pixmap.Bounds().Dy(),
		out_buf: pixmap.Pix,
	}
	width_tiles := (ctx.width + WIDE_TILE_WIDTH - 1) / WIDE_TILE_WIDTH
	height_tiles := (ctx.height + STRIP_HEIGHT - 1) / STRIP_HEIGHT
	for y := range height_tiles {
		for x := range width_tiles {
			tile := &ctx.tiles[y*width_tiles+x]
			fine.clear(tile.bg)
			for i := range tile.cmds {
				cmd := tile.cmds[i]
				fine.run_cmd(cmd, ctx.alphas)
			}
			fine.pack(x, y)
		}
	}
}

func (ctx *CsRenderCtx) render_path(color [4]float32) {
	// XXX support a brush

	// TODO: need to make sure tiles contained in viewport - we'll likely
	// panic otherwise.
	makeTiles(ctx.line_buf, &ctx.tile_buf)
	slices.SortFunc(ctx.tile_buf, tile.cmp)
	// for i, t := range ctx.tile_buf {
	// 	if t == (tile{70, 24, 4294967295, 429490176}) {
	// 		ctx.tile_buf[i] = tile{70, 24, 1, 429490176}
	// 	}
	// }
	renderStripsScalar(ctx.tile_buf, &ctx.strip_buf, &ctx.alphas)
	width_tiles := (ctx.width + WIDE_TILE_WIDTH - 1) / WIDE_TILE_WIDTH
	// XXX can this be a range over ctx.strip_buf or does its length change during the loop?
	for i := range len(ctx.strip_buf) - 1 {
		strip := &ctx.strip_buf[i]

		// Don't render strips that are outside the viewport vertically.
		if int(strip.y()) >= ctx.height {
			break
		}

		next_strip := &ctx.strip_buf[i+1]
		x0 := strip.x()
		y := strip.strip_y()
		row_start := int(y) * width_tiles
		strip_width := next_strip.col - strip.col
		x1 := x0 + strip_width
		xtile0 := int(x0) / WIDE_TILE_WIDTH
		xtile1 := (int(x1) + WIDE_TILE_WIDTH - 1) / WIDE_TILE_WIDTH
		x := x0
		col := strip.col
		for xtile := xtile0; xtile < xtile1; xtile++ {
			x_tile_rel := x % WIDE_TILE_WIDTH
			width := min(x1, uint32((xtile+1)*WIDE_TILE_WIDTH)) - x
			cmd := cmdStrip{
				x:        x_tile_rel,
				width:    width,
				alphaIdx: int(col),
				color:    color,
			}
			x += width
			col += width
			ctx.tiles[row_start+xtile].push(cmd)
		}
		if next_strip.winding != 0 && y == next_strip.strip_y() {
			x = x1
			x2 := next_strip.x()
			fxt0 := x1 / WIDE_TILE_WIDTH
			fxt1 := (x2 + WIDE_TILE_WIDTH - 1) / WIDE_TILE_WIDTH
			for xtile := fxt0; xtile < fxt1; xtile++ {
				x_tile_rel := x % WIDE_TILE_WIDTH
				width := min(x2, ((xtile+1)*WIDE_TILE_WIDTH)) - x
				x += width
				ctx.tiles[row_start+int(xtile)].fill(x_tile_rel, width, color)
			}
		}
	}
}

// impl CsRenderCtx {
//     pub fn tile_stats(&self) {
//         let mut histo = BTreeMap::new();
//         let mut total = 0;
//         for tile in &self.tiles {
//             let count = tile.cmds.len();
//             total += count;
//             *histo.entry(count).or_insert(0) += 1;
//         }
//         println!("total = {total}, {histo:?}");
//     }

//     pub fn debug_dump(&self) {
//         let width_tiles = (self.width + WIDE_TILE_WIDTH - 1) / WIDE_TILE_WIDTH;
//         for (i, tile) in self.tiles.iter().enumerate() {
//             if !tile.cmds.is_empty() || tile.bg.components[3] != 0.0 {
//                 let x = i % width_tiles;
//                 let y = i / width_tiles;
//                 println!("tile {x}, {y} bg {}", tile.bg.to_rgba8());
//                 for cmd in &tile.cmds {
//                     println!("{cmd:?}")
//                 }
//             }
//         }
//     }
// }

func (ctx *CsRenderCtx) setAffine(aff curve.Affine) {
	ctx.transform = aff
}

func (ctx *CsRenderCtx) getAffine() curve.Affine {
	return ctx.transform
}

func (ctx *CsRenderCtx) Fill(path curve.BezPath, color [4]float32) {
	// XXX support brushes
	affine := ctx.getAffine()
	fill(path, affine, &ctx.line_buf)
	ctx.render_path(color)
}

func (ctx *CsRenderCtx) Stroke(path curve.BezPath, stroke_ curve.Stroke, color [4]float32) {
	// XXX support brushes
	affine := ctx.getAffine()
	stroke(path, stroke_, affine, &ctx.line_buf)
	ctx.render_path(color)
}

// impl RenderCtx for CsRenderCtx {
//     type Resource = CsResourceCtx;

//     fn playback(
//         &mut self,
//         recording: &std::sync::Arc<<Self::Resource as piet_next::ResourceCtx>::Recording>,
//     ) {
//         recording.play(self);
//     }
// }

// impl ResourceCtx for CsResourceCtx {
//     type Image = ();

//     type Recording = GenericRecorder<CsRenderCtx>;

//     type Record = GenericRecorder<CsRenderCtx>;

//     fn record(&mut self) -> Self::Record {
//         GenericRecorder::new()
//     }

//     fn make_image_with_stride(
//         &mut self,
//         width: usize,
//         height: usize,
//         stride: usize,
//         buf: &[u8],
//         format: piet_next::ImageFormat,
//     ) -> Result<Self::Image, piet_next::Error> {
//         todo!()
//     }
// }
