FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ingestor ./cmd/main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/ingestor /ingestor
EXPOSE 8080
ENTRYPOINT ["/ingestor"]
