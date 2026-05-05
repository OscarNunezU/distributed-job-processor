FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/bin/worker .
EXPOSE 9090
ENTRYPOINT ["/app/worker"]
