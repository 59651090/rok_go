FROM golang:1.21-alpine AS builder

# 国内代理，加速下载依赖
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

# 先复制 go.mod go.sum，缓存依赖层
COPY go.mod go.sum ./
RUN go mod download

# 复制源码编译（纯 Go sqlite，不需要 CGO）
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o ironfist-server main.go

# 运行阶段（用更小的 alpine
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段拷贝二进制
COPY --from=builder /app/ironfist-server /app/ironfist-server

# 数据卷：截图和数据库放外面，方便持久化
VOLUME ["/app/uploads", "/app/data"]

# 数据库放到 data 目录
ENV DB_PATH=/app/data/ironfist.db
ENV UPLOAD_DIR=/app/uploads

EXPOSE 14742

CMD ["/app/ironfist-server"]
