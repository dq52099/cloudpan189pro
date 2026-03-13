# Stage 1: Build the Vue frontend
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder

WORKDIR /app/fe

# 使用中国镜像源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 设置 npm 为淘宝镜像
RUN npm config set registry https://registry.npmmirror.com

RUN corepack enable && corepack prepare pnpm@latest --activate
# 设置 pnpm 为淘宝镜像
RUN pnpm config set registry https://registry.npmmirror.com

# 先复制 package.json 和 lock 文件
COPY fe/package.json fe/pnpm-lock.yaml ./
# 先不安装依赖，等复制源码后再安装（避免平台问题）

# 复制源码
COPY fe/ ./

# 在容器内安装依赖
RUN pnpm install --frozen-lockfile

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
RUN go mod tidy
RUN go mod download

COPY . .
# Copy frontend static files from frontend-builder stage
COPY --from=frontend-builder /app/fe/dist ./fe/dist

# 构建参数 - 与 Makefile 保持一致
ARG MODULE_NAME=github.com/xxcheng123/cloudpan189-share
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
    go mod tidy && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 \
    go build \
    -ldflags="-s -w \
              -X ${MODULE_NAME}/configs.Commit=${VAR_COMMIT} \
              -X ${MODULE_NAME}/configs.BuildDate=${BUILD_DATE} \
              -X ${MODULE_NAME}/configs.GitSummary=${VAR_GIT_SUMMARY} \
              -X ${MODULE_NAME}/configs.GitBranch=${VAR_GIT_BRANCH}" \
    -o ${OUTPUT_DIR}/${BINARY_NAME} ./cmd/main.go

# Stage 3: Final image
FROM --platform=$TARGETPLATFORM alpine:latest

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
