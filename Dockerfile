# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache build-base git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags='-s -w -extldflags "-static"' -o openveth-api ./cmd/openveth-api

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache \
    iproute2 \
    iptables \
    iputils \
    docker-cli

WORKDIR /app

COPY --from=builder /app/openveth-api .

EXPOSE 8080

CMD ["./openveth-api"]
