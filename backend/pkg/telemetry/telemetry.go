package telemetry

// Telemetry initialises structured logging (slog) and the Prometheus metrics registry.
// Exposes GET /metrics for Grafana scraping and GET /healthz as a k8s readiness probe.
