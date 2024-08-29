package repositories

type Collection[T any] struct {
	Data       []T
	Count      uint64
	Offset     uint64
	Limit      uint64
	TotalCount uint64
}

type Bounds struct {
	MinLon float64
	MaxLon float64
	MinLat float64
	MaxLat float64
}
