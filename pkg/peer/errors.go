package peer

type ContextErrorAggregator interface {
	Collect(error)
}
