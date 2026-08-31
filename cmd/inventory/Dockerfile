FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/inventory ./cmd/inventory

FROM alpine:latest
WORKDIR /app
COPY --from=builder /bin/inventory /app/inventory
EXPOSE 50051
CMD ["/app/inventory"]