// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package ucdtrie

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"unsafe"

	"golang.org/x/exp/constraints"
	"honnef.co/go/safeish"
)

func (seq Compressed[T]) Code(dataVar, indexVar string) []byte {
	f := &bytes.Buffer{}

	fmt.Fprintf(f, "var %s = [0x10000]uint8{\n", dataVar)
	for blockID, block := range seq.Blocks {
		fmt.Fprintf(f, "// block %d (%#x - %#x)\n",
			blockID, blockID*len(block), (blockID+1)*len(block))
		n := 0
		for offset, datum := range block {
			if datum == *new(T) && n == 0 {
				continue
			}
			n = (n + 1) % 4
			fmt.Fprintf(f, "%#04x: %#02x, ", blockID*len(block)+offset, datum)
			if n == 0 {
				fmt.Fprintln(f)
			}
		}
		if n != 0 {
			fmt.Fprintln(f)
		}
		fmt.Fprintln(f)
	}
	fmt.Fprintln(f, "}")

	fmt.Fprintf(f, "var %s = [0x2000]uint8{\n", indexVar)
	n := 0
	for i, id := range seq.BlockIDs {
		n = (n + 1) % 4
		fmt.Fprintf(f, "%#04x: %#02x, ", i, id)
		if n == 0 {
			fmt.Fprintln(f)
		}
	}
	if n != 0 {
		fmt.Fprintln(f)
	}
	fmt.Fprintln(f, "}")

	return f.Bytes()
}

func (seq Compressed[T]) PrintStats() {
	dataSize := seq.BlockSize * int(unsafe.Sizeof(*new(T))) * len(seq.Blocks)
	indexSize := len(seq.BlockIDs)
	totalSize := dataSize + indexSize

	rawSize := seq.BlockSize * len(seq.BlockIDs) * int(unsafe.Sizeof(*new(T)))

	fmt.Fprintf(os.Stderr, "blocks: %d\n", len(seq.BlockIDs))
	fmt.Fprintf(os.Stderr, "unique blocks: %d\n", len(seq.Blocks))
	fmt.Fprintf(os.Stderr, "%d bytes for data\n", dataSize)
	fmt.Fprintf(os.Stderr, "%d bytes for indices into data\n", indexSize)
	fmt.Fprintf(os.Stderr, "%d bytes total size\n", totalSize)
	fmt.Fprintf(os.Stderr, "%d bytes for uncompressed data\n", rawSize)
	fmt.Fprintf(os.Stderr, "%f%% compression ratio\n", (1-float64(totalSize)/float64(rawSize))*100)
}

type Compressed[T constraints.Integer] struct {
	BlockSize int
	Blocks    [][]T
	BlockIDs  []int
}

func Compress[T constraints.Integer](seq []T, blockSize int) Compressed[T] {
	var blocks [][]T
	dedup := map[string]int{}
	var blockUses []int

	keyBlock := make([]T, blockSize)
	addBlock := func(curBlock []T) {
		clear(keyBlock)
		copy(keyBlock, curBlock)
		key := safeish.SliceCast[[]byte](keyBlock)
		key_ := string(key)
		if id, ok := dedup[key_]; ok {
			blockUses = append(blockUses, id)
		} else {
			blocks = append(blocks, curBlock)
			id := len(blocks) - 1
			dedup[key_] = id
			blockUses = append(blockUses, id)
		}
	}
	for block := range slices.Chunk(seq, blockSize) {
		addBlock(block)
	}

	return Compressed[T]{
		BlockSize: blockSize,
		Blocks:    blocks,
		BlockIDs:  blockUses,
	}
}
