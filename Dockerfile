FROM golang:1.27-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o tourneyweb .

FROM alpine:3.24

RUN addgroup -S tourneyweb && adduser -S -G tourneyweb tourneyweb

WORKDIR /app
COPY --from=builder /build/tourneyweb .

USER tourneyweb

EXPOSE 8989

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8989/healthz || exit 1

ENTRYPOINT ["./tourneyweb"]
