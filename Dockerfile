# 使用官方 Go 镜像作为基础镜像
FROM golang:1.22 AS builder

# 设置工作目录
WORKDIR /app

# 将当前目录的所有文件复制到容器的 /app 目录
COPY . .

# 下载 Go 的依赖（如果有的话）
RUN go mod tidy

# 编译 Go 程序
RUN go build -tags netgo -o main .

FROM alpine:latest
WORKDIR /root
COPY --from=builder /app/main .
# 容器启动时运行 Go 程序
CMD ["./main"]

# 暴露 8080 端口
EXPOSE 8080
