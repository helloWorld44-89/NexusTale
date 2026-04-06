package collaboration

// Hub is the in-process WebSocket connection manager.
// It broadcasts document operations to all connected clients in a project room.
// For multi-pod k8s deployments, operations are fanned out via Redis pub/sub
// so every pod's Hub receives them (see pkg/cache for the pub/sub interface).
