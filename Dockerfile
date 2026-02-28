# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tradebot ./cmd/tradebot

# Stage 2: Runtime
FROM alpine:latest

RUN addgroup -S tradebot && adduser -S tradebot -G tradebot
RUN mkdir -p /data /etc/tradebot && chown tradebot:tradebot /data

COPY --from=builder /tradebot /tradebot
COPY config.yaml /etc/tradebot/config.yaml

EXPOSE 8080
VOLUME /data

USER tradebot
ENTRYPOINT ["/tradebot"]
CMD ["-config", "/etc/tradebot/config.yaml"]
