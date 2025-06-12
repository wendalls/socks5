# 使用官方 Go 镜像作为基础镜像
FROM golang:1.22 AS builder

# 设置工作目录
WORKDIR /app

# 将当前目录的所有文件复制到容器的 /app 目录
COPY . .

# 下载 Go 的依赖（如果有的话）
RUN go mod tidy

# 编译 Go 程序
RUN go build -tags netgo -ldflags="-s -w" -o main .

FROM alpine:latest
WORKDIR /root
COPY --from=builder /app/main .
COPY --from=builder /app/cert.pem .
COPY --from=builder /app/key.pem .
# 容器启动时运行 Go 程序
ENV TOKEN="secret_token_123"
CMD ["./main", "-token", "$TOKEN"]

# 暴露 443 端口
EXPOSE 443
