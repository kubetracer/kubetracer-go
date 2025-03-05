FROM golang:1.24-bullseye as builder

WORKDIR /app
COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o webhook main.go

FROM debian:bullseye-slim

WORKDIR /app
COPY --from=builder /app/webhook /webhook
COPY --from=builder /app/certs /certs

USER nonroot:nonroot

ENTRYPOINT ["/webhook"]