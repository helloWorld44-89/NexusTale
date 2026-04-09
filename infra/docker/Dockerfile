# Stage 1: build the Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache module downloads separately from source
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api

# Stage 2: minimal runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /api /api
# Migrations are embedded in the image so the runner can apply them on startup
COPY backend/pkg/db/migrations /app/pkg/db/migrations

EXPOSE 8080

ENTRYPOINT ["/api"]
