package peer

// ContextErrorAggregator collect errors over context life time
type ContextErrorAggregator interface {
	Collect(error)
}
