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

# 创建日志目录
mkdir -p logs

# 查找并停止旧进程
if command -v lsof &> /dev/null; then
    PID=$(lsof -ti:12395)
elif command -v netstat &> /dev/null; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        PID=$(netstat -an | grep ":12395" | grep LISTEN | awk '{print $NF}')
    else
        PID=$(netstat -ano | grep ":12395" | grep LISTEN | head -1 | awk '{print $NF}')
    fi
fi

if [ -n "$PID" ]; then
    echo "停止旧进程: $PID"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" || "$OSTYPE" == "cygwin" ]]; then
        taskkill /F /PID $PID 2>/dev/null
    else
        kill -9 $PID 2>/dev/null
    fi
    sleep 2
fi

# 启动新服务
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" || "$OSTYPE" == "cygwin" ]]; then
    start /b "" ./share.exe
    echo "服务已启动 (Windows后台)"
else
    nohup ./share.exe > logs/share.log 2>&1 &
    echo "服务已启动，PID: $!"
fi

# 等待启动
sleep 3

# 验证
if [ -f "logs/share.log" ]; then
    if grep -q "system running" logs/share.log 2>/dev/null; then
        echo "✅ 服务启动成功！"
    else
        echo "⚠️ 请检查日志 logs/share.log"
    fi
else
    echo "✅ 服务已启动"
fi
