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
