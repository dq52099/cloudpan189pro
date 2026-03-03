# 项目编译构建指南

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
npm run build
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

### 查找当前进程
```bash
# Windows
netstat -ano | findstr ":12395 "

# Linux
lsof -i :12395
```

### 停止旧进程
```bash
# Windows - 使用 PID
taskkill /F /PID <PID>

# Linux
kill -9 <PID>
```

### 启动新服务
```bash
# Windows
start /b "" ./share.exe

# Linux
./share.exe &

# 或使用 nohup
nohup ./share.exe > logs/share.log 2>&1 &
```

### 验证启动成功
```bash
# 检查日志
tail -10 logs/share.log

# 应该看到 "system running...."
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

---

## 完整示例：添加新功能后的构建流程

```bash
# 1. 修改代码...

# 2. 构建前端
cd fe && npm run build

# 3. 编译后端
cd .. && go build -o share.exe ./cmd/main.go

# 4. 重启服务
# 4.1 查找进程
netstat -ano | findstr ":12395 "
# 4.2 停止旧进程
taskkill /F /PID <PID>
# 4.3 启动新进程
start /b "" ./share.exe

# 5. 验证
tail -10 logs/share.log
```

---

## 快速命令汇总

```bash
# 完整构建流程（单行）
cd fe && npm run build && cd .. && go build -o share.exe ./cmd/main.go
```
