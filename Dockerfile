# ---------- Build stage ----------
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o customers ./cmd/customers

# ---------- Runtime stage ----------
FROM alpine:3.22

WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/customers /app/customers

RUN chown appuser:appgroup /app/customers \
    && chmod +x /app/customers

USER appuser

ENTRYPOINT ["/app/customers"]
