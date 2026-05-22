# 项目编译构建指南

## 开发流程

### 1. 构建前端
```bash
cd fe
corepack pnpm@9.15.9 install --frozen-lockfile
corepack pnpm@9.15.9 build
```

### 2. 编译后端
```bash
go build -o output/share.exe ./cmd/main.go
```

### 3. 检查服务状态
```bash
# 查找进程
netstat -ano | findstr ":12395 " | findstr LISTENING

# 或
tasklist | findstr share
```

### 4. 停止旧服务
```bash
taskkill //F //IM share.exe
# 或
taskkill //F //PID <PID>
```

### 5. 启动新服务
```bash
# 前台运行（查看日志）
./output/share.exe

# 后台运行
./output/share.exe > logs/share.log 2>&1 &
```

### 6. 验证服务运行
```bash
# 检查端口
netstat -ano | findstr ":12395 " | findstr LISTENING

# 检查日志
type logs\share.log
```

---

## 一键启动（推荐）

直接在项目根目录运行：

```bat
cd fe && corepack pnpm@9.15.9 install --frozen-lockfile && corepack pnpm@9.15.9 build && cd .. && go build -o output/share.exe ./cmd/main.go && taskkill //F //IM share.exe 2>nul & timeout /t 1 /nobreak >nul & output/share.exe > logs/share.log 2>&1
```

或者分步执行：
```bat
cd fe
corepack pnpm@9.15.9 install --frozen-lockfile
corepack pnpm@9.15.9 build
cd ..
go build -o output/share.exe ./cmd/main.go
taskkill //F //IM share.exe 2>nul
timeout /t 1 /nobreak >nul
output/share.exe > logs/share.log 2>&1
```

---

```bash
# 构建前端 -> 编译后端 -> 重启服务
cd fe && corepack pnpm@9.15.9 install --frozen-lockfile && corepack pnpm@9.15.9 build && cd .. && go build -o output/share.exe ./cmd/main.go && taskkill //F //IM share.exe 2>nul & sleep 1 && ./output/share.exe > logs/share.log 2>&1 &
```

---

## 前置说明

- 前端使用 Vue.js + Vite
- 后端使用 Go + Gin
- 前端代码通过 `embed` 嵌入到后端二进制文件中

---

## 步骤 1：构建前端

```bash
# 进入前端目录
cd fe

# 构建生产版本
corepack pnpm@9.15.9 install --frozen-lockfile
corepack pnpm@9.15.9 build
```

**验证构建成功**：
```bash
ls -la fe/dist/index.html
# 应该看到最新的时间戳
```

---

## 步骤 2：编译后端

```bash
# 进入项目根目录
cd /path/to/crisp-island

# 编译后端（会自动嵌入 fe/dist 目录）
go build -o share.exe ./cmd/main.go

# 验证
ls -la share.exe
```

---

## 步骤 3：重启服务

### 方式一：一键构建并重启（推荐）

```bash
# 创建构建脚本
cat > build_and_restart.sh << 'EOF'
#!/bin/bash

echo "=== 开始构建 ==="

# 1. 构建前端
echo "[1/3] 构建前端..."
cd fe
corepack pnpm@9.15.9 install --frozen-lockfile
corepack pnpm@9.15.9 build
cd ..

# 2. 构建后端
echo "[2/3] 编译后端..."
go build -o share.exe ./cmd/main.go

# 3. 重启服务
echo "[3/3] 重启服务..."

# 查找并停止旧进程
if command -v lsof &> /dev/null; then
    PID=$(lsof -ti:12395)
elif command -v netstat &> /dev/null; then
    PID=$(netstat -ano | grep ":12395" | head -1 | awk '{print $NF}')
fi

if [ -n "$PID" ]; then
    echo "停止旧进程: $PID"
    kill -9 $PID 2>/dev/null || taskkill /F /PID $PID 2>/dev/null
    sleep 2
fi

# 启动新服务
mkdir -p logs
nohup <编译后的程序> > logs/share.log 2>&1 &
echo "服务已启动，PID: $!"

# 等待启动
sleep 3

# 验证
if grep -q "system running" logs/share.log 2>/dev/null; then
    echo "✅ 服务启动成功！"
else
    echo "⚠️ 请检查日志 logs/share.log"
fi
EOF

# 运行
chmod +x build_and_restart.sh
./build_and_restart.sh
```

### 方式二：手动重启

#### 查找当前进程
```bash
# Windows
netstat -ano | findstr ":12395 "

# Linux
lsof -i :12395
```

#### 停止旧进程
```bash
# Windows - 使用 PID
taskkill /F /PID <PID>

# Linux
kill -9 <PID>
```

#### 启动新服务
```bash
# Windows
start /b "" <编译后的程序>

# Linux
$ ./share.exe &

# 或使用 nohup
nohup <编译后的程序> > logs/share.log 2>&1 &
```

#### 验证启动成功
```bash
# 检查日志
tail -10 logs/share.log

# 应该看到 "system running...."
```

---

## Docker 部署

### 快速启动（SQLite 版本）

```bash
# 1. 创建目录
mkdir -p cloudpan189pro/{etc,data,logs,media_dir}

# 2. 创建 docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  app:
    image: dq52099/cloudpan189pro:latest
    container_name: cloudpan189pro
    restart: unless-stopped
    ports:
      - "12395:12395"
    volumes:
      - ./etc/config.yaml:/app/etc/config.yaml:ro
      - ./data:/app/data
      - ./logs:/app/logs
      - ./media_dir:/app/media_dir
    environment:
      - TZ=Asia/Shanghai
EOF

# 3. 创建 config.yaml
cat > etc/config.yaml << 'EOF'
port: 12395
dbFile: "data/data.db"
logFile: "logs/share.log"
mediaDir: "media_dir"
dbType: "sqlite"
EOF

# 4. 启动服务
docker-compose up -d
```

### PostgreSQL 版本

```yaml
services:
  app:
    image: dq52099/cloudpan189pro:latest
    container_name: cloudpan189pro
    restart: unless-stopped
    ports:
      - "12395:12395"
    volumes:
      - ./etc/config.yaml:/app/etc/config.yaml:ro
      - ./data:/app/data
      - ./logs:/app/logs
      - ./media_dir:/app/media_dir
    environment:
      - TZ=Asia/Shanghai
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - cloudpan189

  postgres:
    image: postgres:15-alpine
    container_name: cloudpan189pro-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=share
    volumes:
      - ./postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - cloudpan189

networks:
  cloudpan189:
    driver: bridge
```

---

## 配置文件说明

### config.yaml（SQLite 默认）
```yaml
port: 12395
dbFile: "data/data.db"
logFile: "logs/share.log"
mediaDir: "media_dir"
# 数据库类型: sqlite, mysql, postgresql
dbType: "sqlite"
```

### 切换数据库
修改 `dbType` 为 `sqlite`、`mysql` 或 `postgresql`，并配置对应的连接信息。

---

## 构建 Docker 镜像

```bash
# 克隆代码
git clone https://github.com/dq52099/cloudpan189pro.git
cd cloudpan189pro

# 构建镜像
docker build -t dq52099/cloudpan189pro:latest .

# 推送镜像
docker push dq52099/cloudpan189pro:latest
```

---

## 常见问题

### Q1: 修改了代码但不生效

1. 确认前端是否重新构建：`ls -la fe/dist/index.html`
2. 确认后端是否重新编译：`ls -la share.exe`
3. 确认旧进程是否已停止：`netstat -ano | findstr ":12395 "`
4. 确认新进程是否启动：检查日志

### Q2: API 返回数据缺少新字段

1. 检查 Go 模型定义是否有该字段（检查 `repository/models/` 下的文件）
2. 检查数据库表结构是否有该字段
3. 重新编译后端

### Q3: 前端页面没有更新

1. 使用无痕模式测试
2. 强制刷新：`Ctrl+Shift+R`（Chrome）
3. 清除浏览器缓存

### Q4: Docker 部署后数据持久化

确保挂载了以下卷：
- `./data` - 数据库文件
- `./media_dir` - 媒体文件
- `./logs` - 日志文件

---

## 飞牛NAS 部署

```bash
# 1. 在飞牛NAS上创建项目目录
mkdir -p /mnt/storage/appdata/cloudpan189pro/{etc,data,logs,media_dir}

# 2. 复制配置文件到挂载目录
# 编辑 etc/config.yaml 确保 dbType: "sqlite"

# 3. 启动容器
docker run -d \
  --name cloudpan189pro \
  --restart unless-stopped \
  -p 12395:12395 \
  -v /mnt/storage/appdata/cloudpan189pro/etc:/app/etc:ro \
  -v /mnt/storage/appdata/cloudpan189pro/data:/app/data \
  -v /mnt/storage/appdata/cloudpan189pro/logs:/app/logs \
  -v /mnt/storage/appdata/cloudpan189pro/media_dir:/app/media_dir \
  -e TZ=Asia/Shanghai \
  dq52099/cloudpan189pro:latest
```

或使用 docker-compose：
```bash
# 创建目录
mkdir -p /mnt/storage/appdata/cloudpan189pro/{etc,data,logs,media_dir}

# 复制 docker-compose.yml 和 config.yaml

# 启动
cd /mnt/storage/appdata/cloudpan189pro
docker-compose up -d
```
