# Stage 1: Build the Vue frontend
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder

WORKDIR /app/fe

# 使用中国镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 设置 npm 为淘宝镜像
RUN npm config set registry https://registry.npmmirror.com

ARG PNPM_VERSION=9.15.9
RUN corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate
# 设置 pnpm 为淘宝镜像
RUN pnpm config set registry https://registry.npmmirror.com

# 先复制 package.json 和 lock 文件
COPY fe/package.json fe/pnpm-lock.yaml ./

# 先安装依赖以复用 Docker 缓存；.dockerignore 会排除本地 node_modules。
RUN pnpm install --frozen-lockfile

# 复制源码并构建前端
COPY fe/ ./
RUN pnpm build

# Stage 2: Build the Go backend
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend-builder

# 添加构建参数
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# 使用中国镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 设置 Go 代理为中国镜像
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy frontend static files from frontend-builder stage
COPY --from=frontend-builder /app/fe/dist ./fe/dist

# 构建参数 - 与 Makefile 保持一致
ARG MODULE_NAME=github.com/xxcheng123/cloudpan189-share
ARG CONFIG_PACKAGE=github.com/xxcheng123/cloudpan189-share/internal/configs
ARG VAR_COMMIT
ARG VAR_BUILD_DATE
ARG VAR_GIT_SUMMARY
ARG VAR_GIT_BRANCH
ARG OUTPUT_DIR=/app
ARG BINARY_NAME=share

# 构建应用 - 使用与 Makefile 相同的参数
RUN BUILD_DATE="${VAR_BUILD_DATE}" && \
    if [ -z "$BUILD_DATE" ]; then BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"; fi && \
    echo "Building for $TARGETOS/$TARGETARCH on $BUILDPLATFORM" && \
    echo "Build date: $BUILD_DATE" && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 \
    go build \
    -ldflags="-s -w \
              -X ${CONFIG_PACKAGE}.Commit=${VAR_COMMIT} \
              -X ${CONFIG_PACKAGE}.BuildDate=${BUILD_DATE} \
              -X ${CONFIG_PACKAGE}.GitSummary=${VAR_GIT_SUMMARY} \
              -X ${CONFIG_PACKAGE}.GitBranch=${VAR_GIT_BRANCH}" \
    -o ${OUTPUT_DIR}/${BINARY_NAME} ./cmd/main.go

# Stage 3: Final image
FROM alpine:latest

WORKDIR /app

# 使用中国镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 设置时区
ENV TZ=Asia/Shanghai
ENV GIN_MODE=release
RUN cat /etc/apk/repositories \
  && apk update \
  && apk add --no-cache ca-certificates tzdata wget

# Copy backend executable from backend-builder stage
COPY --from=backend-builder /app/share .

# Copy configuration file
COPY etc/config.yaml ./etc/config.yaml

# 创建数据目录
RUN mkdir -p /app/data

# Expose the port the application runs on (from config.yaml, default 12395)
EXPOSE 12395

# 添加健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:12395/ || exit 1

# Command to run the application
CMD ["./share", "-config", "./etc/config.yaml"]
