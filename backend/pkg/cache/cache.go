package cache

// Cache is the interface used throughout the service layer.
// InMemory backs it with a sync.Map LRU for local dev (no Redis required).
// Redis backs it for production — drop-in via the same interface.
// This design lets you defer the Redis dependency until query latency justifies it.
