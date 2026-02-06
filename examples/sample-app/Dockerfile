# Build stage
FROM golang:1.24.4-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/app .

# Set environment variables (can be overridden at runtime)
ENV KAFKA_BROKERS=localhost:9092
ENV KAFKA_TOPIC=test-topic
ENV KAFKA_GROUP_ID=sample-consumer-group

EXPOSE 8080

# Default to consumer mode; override with --mode=producer
ENTRYPOINT ["./app"]
CMD ["--mode=consumer"]
