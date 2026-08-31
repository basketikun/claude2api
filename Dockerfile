FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/claude2api ./cmd/server

FROM alpine:3.24
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/claude2api .
COPY config.example.yaml config.yaml
EXPOSE 8787
CMD ["./claude2api"]
