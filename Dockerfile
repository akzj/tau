# Multi-stage build
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o tau ./cmd/tau/

FROM alpine:3.20
RUN adduser -D -h /tau tauuser
WORKDIR /tau
COPY --from=builder /build/tau .
USER tauuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD ./tau --health-addr :8080 || exit 1
ENTRYPOINT ["./tau"]
