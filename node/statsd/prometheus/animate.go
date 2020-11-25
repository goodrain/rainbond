package prometheus

type Animate interface {
	SetTimestamp(int64)
	GetTimestamp() int64
}
