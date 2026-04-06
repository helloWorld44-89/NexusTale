package wiki

// Timeline management (CreateTimelineEvent, ListTimelineEvents, UpdateTimelineEvent,
// DeleteTimelineEvent) is implemented in service.go.
//
// Future: relative anchoring ("X years after Event Y") would require a recursive
// resolution step here. For now all events use absolute year/month/day values,
// with era as a free-text label for in-world calendar systems.
