// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package mem

type DoubleBuffered[T any] struct {
	Generation  uint32
	Front, Back T
}

func (db *DoubleBuffered[T]) Swap() {
	db.Front, db.Back = db.Back, db.Front
	db.Generation++
}

type DoubleBufferedSlice[T any] struct {
	Generation  uint32
	Front, Back []T
}

func (db *DoubleBufferedSlice[T]) Swap() {
	db.Front, db.Back = db.Back[:0], db.Front[:0]
	db.Generation++
}
