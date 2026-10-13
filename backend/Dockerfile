# Build inside Linux so host binaries, OS and CPU architecture do not leak into the image.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache ca-certificates wget tzdata
COPY --from=build /out/server ./server
# 仅拷贝运行时必需的配置文件（不含本地备份缓存）
COPY config/bootstrap.yaml ./config/bootstrap.yaml
COPY config/rbac_model.conf ./config/rbac_model.conf
COPY config/permissions.yaml ./config/permissions.yaml
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=5 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1
CMD ["./server"]
