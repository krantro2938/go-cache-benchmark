FROM golang:1.25.1-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bench-runner .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app  # ← critical!
COPY --from=builder /app/bench-runner .
# No need to copy results — it's a volume
CMD ["./bench-runner"]
