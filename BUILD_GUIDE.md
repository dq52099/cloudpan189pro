# 项目编译构建指南

## 环境要求

- Go 1.25+
- Node.js 22+
- Corepack
- Docker（可选，用于镜像或 compose 部署）

## 本地构建

```bash
make build
```

等价分步命令：

```bash
corepack pnpm@9.15.9 --dir fe install --frozen-lockfile
corepack pnpm@9.15.9 --dir fe build
mkdir -p output logs
go build -o output/share ./cmd/main.go
```

Windows PowerShell：

```powershell
corepack pnpm@9.15.9 --dir fe install --frozen-lockfile
corepack pnpm@9.15.9 --dir fe build
New-Item -ItemType Directory -Force output, logs | Out-Null
go build -o output/share.exe ./cmd/main.go
```

## 启动服务

Linux / macOS：

```bash
./output/share -config etc/config.yaml
```

后台运行：

```bash
mkdir -p logs
nohup ./output/share -config etc/config.yaml > logs/share.log 2>&1 &
```

Windows PowerShell：

```powershell
.\output\share.exe -config .\etc\config.yaml
```

## 重启已有服务

Linux / macOS：

```bash
pkill -f 'output/share' || true
nohup ./output/share -config etc/config.yaml > logs/share.log 2>&1 &
```

Windows PowerShell：

```powershell
Get-Process share -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Process -FilePath .\output\share.exe -ArgumentList '-config', '.\etc\config.yaml' -RedirectStandardOutput .\logs\share.log -RedirectStandardError .\logs\share.err.log
```

## 验证

```bash
curl -I http://localhost:12395/
```

Web 界面：`http://localhost:12395`

WebDAV：`http://localhost:12395/dav`

## Docker Compose

PostgreSQL 版：

```bash
docker compose up -d
```

SQLite 版：

```bash
cp etc/config.example.yaml etc/config.yaml
docker compose -f docker-compose.sqlite.yml up -d
```

## Docker 镜像

```bash
docker build -t dq52099/cloudpan189pro:latest .
docker run -d \
  --name cloudpan189pro \
  -p 12395:12395 \
  -v "$PWD/data:/app/data" \
  -v "$PWD/logs:/app/logs" \
  -v "$PWD/media_dir:/app/media_dir" \
  --restart unless-stopped \
  dq52099/cloudpan189pro:latest
```

## 多架构二进制

```bash
make build-multi-arch
ls -la output/
```

## 常见问题

### 前端页面没有更新

确认 `fe/dist/index.html` 是最新构建产物，然后重新执行后端构建：

```bash
corepack pnpm@9.15.9 --dir fe build
go build -o output/share ./cmd/main.go
```

### Docker Compose 无法连接 PostgreSQL

默认 `docker-compose.yml` 挂载 `etc/config.docker.yaml`，其中 `postgres.host` 必须是 `postgres`。如果自定义配置，容器内不要使用 `localhost` 连接 compose 的 PostgreSQL 服务。

### SQLite 数据没有持久化

确认挂载了数据目录：

```yaml
volumes:
  - ./data:/app/data
```
