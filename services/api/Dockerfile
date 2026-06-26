# Stage 1: Build
FROM golang:1.25-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o agentforge-api ./cmd/agentforge-api

# Stage 2: Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates docker-cli

WORKDIR /app

COPY --from=builder /build/agentforge-api .
COPY --from=builder /build/migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["./agentforge-api"]
