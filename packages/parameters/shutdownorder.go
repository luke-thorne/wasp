package parameters

const (
	PriorityDatabase = iota

	PriorityChains
	PriorityPeering
	PriorityNodeConnection
	PriorityWebAPI
	PriorityDBGarbageCollection
	PriorityPrometheus
)
