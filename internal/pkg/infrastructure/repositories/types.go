package repositories

type Collection[T any] struct {
	Data       []T
	Count      uint64
	Offset     uint64
	Limit      uint64
	TotalCount uint64
}
