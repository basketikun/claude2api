FROM golang:1.25-alpine AS builder
# 国内网络下 proxy.golang.org 常常 TLS 超时，允许用 build-arg 切镜像源
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY VERSION CHANGELOG.md version.go ./
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
