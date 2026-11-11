FROM golang:1.25.1-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bench-runner .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bench-runner .
# Results are written to the /app/results volume.
CMD ["./bench-runner"]
