package adapters

// Adapter is the common interface all AI backends must satisfy.
// Complete sends a prompt and streams tokens via the provided channel.
// Models returns the list of available model identifiers for this backend.
