// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package mem

type DoubleBuffered[T any] struct {
	Front, Back T
}

func (db *DoubleBuffered[T]) Swap() {
	db.Front, db.Back = db.Back, db.Front
}

type DoubleBufferedSlice[T any] struct {
	Front, Back []T
}

func (db *DoubleBufferedSlice[T]) Swap() {
	db.Front, db.Back = db.Back[:0], db.Front[:0]
}

func CopyMap[K comparable, V any, M ~map[K]V](m M) M {
	out := make(M)
	for k, v := range m {
		out[k] = v
	}
	return out
}
