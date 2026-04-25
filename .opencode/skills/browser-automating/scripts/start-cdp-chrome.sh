#!/bin/bash
# 启动 Chrome CDP 调试模式
# 支持自定义端口和URL参数
# 适用于 Deepin / Linux / macOS

set -e

# 默认端口
CDP_PORT=${CDP_PORT:-9223}
USER_DATA_DIR="$HOME/.config/chrome-cdp-debug"
START_URL="${1:-about:blank}"

echo "=========================================="
echo " 启动 Chrome CDP 调试模式"
echo "=========================================="
echo ""

# ─── 检测Chrome路径 ────────────────────────────────────────────────
detect_chrome() {
    # Linux路径（按优先级）
    local linux_paths=(
        "/opt/apps/cn.google.chrome-pre/files/google/chrome/google-chrome"  # Deepin
        "/usr/bin/google-chrome-stable"
        "/opt/google/chrome/google-chrome"
        "/usr/bin/google-chrome"
        "/usr/bin/chromium"
        "/usr/bin/chromium-browser"
        "/snap/bin/chromium"
    )
    
    # macOS路径
    local mac_paths=(
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
        "/Applications/Chromium.app/Contents/MacOS/Chromium"
    )
    
    case "$(uname -s)" in
        Darwin)
            for p in "${mac_paths[@]}"; do
                [ -f "$p" ] && echo "$p" && return
            done
            command -v google-chrome >/dev/null 2>&1 && echo "google-chrome" && return
            ;;
        Linux)
            for p in "${linux_paths[@]}"; do
                [ -f "$p" ] && echo "$p" && return
            done
            for cmd in google-chrome-stable google-chrome chromium chromium-browser; do
                command -v "$cmd" >/dev/null 2>&1 && echo "$cmd" && return
            done
            ;;
    esac
    echo ""
}

CHROME_PATH=$(detect_chrome)

if [ -z "$CHROME_PATH" ]; then
    echo "✗ 未找到 Chrome/Chromium"
    echo ""
    echo "安装方式："
    echo "  Deepin: 应用商店搜索 Google Chrome"
    echo "  Ubuntu: sudo apt install google-chrome-stable"
    echo "  macOS:  下载 https://www.google.com/chrome/"
    exit 1
fi

echo "Chrome: $CHROME_PATH"
echo "端口: $CDP_PORT"
echo "用户数据: $USER_DATA_DIR"
echo ""

# ─── 检查是否已运行 ────────────────────────────────────────────────
if curl -s "http://localhost:${CDP_PORT}/json/version" > /dev/null 2>&1; then
    VERSION=$(curl -s "http://localhost:${CDP_PORT}/json/version" 2>/dev/null | grep -o '"Browser":"[^"]*"' | cut -d'"' -f4 || echo "未知")
    echo "✓ CDP 已运行于端口 ${CDP_PORT}"
    echo "  Version: $VERSION"
    echo "  CDP URL: http://localhost:${CDP_PORT}"
    echo ""
    
    if [ "$START_URL" != "about:blank" ]; then
        echo "正在打开: $START_URL"
        "$CHROME_PATH" --remote-debugging-port="${CDP_PORT}" --user-data-dir="$USER_DATA_DIR" "$START_URL" > /dev/null 2>&1 &
    fi
    
    echo "停止命令: pkill -f 'remote-debugging-port=${CDP_PORT}'"
    exit 0
fi

# ─── 关闭已有实例（端口冲突）────────────────────────────────────────
if pgrep -f "remote-debugging-port=${CDP_PORT}" > /dev/null 2>&1; then
    echo "正在关闭现有 CDP 实例..."
    pkill -f "remote-debugging-port=${CDP_PORT}" 2>/dev/null || true
    sleep 2
    
    if pgrep -f "remote-debugging-port=${CDP_PORT}" > /dev/null 2>&1; then
        pkill -9 -f "remote-debugging-port=${CDP_PORT}" 2>/dev/null || true
        sleep 1
    fi
fi

# ─── 创建用户数据目录 ────────────────────────────────────────────────
mkdir -p "$USER_DATA_DIR"

# ─── 启动Chrome ──────────────────────────────────────────────────────
echo "正在启动 Chrome CDP..."
echo ""

TMP_LOG="/tmp/chrome-cdp-${CDP_PORT}.log"

"$CHROME_PATH" \
    --remote-debugging-port="${CDP_PORT}" \
    --user-data-dir="$USER_DATA_DIR" \
    --no-first-run \
    --no-default-browser-check \
    --disable-background-networking \
    --disable-sync \
    --disable-translate \
    --disable-features=TranslateUI \
    --remote-allow-origins=* \
    "$START_URL" \
    > "$TMP_LOG" 2>&1 &

CHROME_PID=$!

# ─── 等待启动 ────────────────────────────────────────────────────────
echo "等待 Chrome 启动..."
for i in {1..20}; do
    if curl -s "http://localhost:${CDP_PORT}/json/version" > /dev/null 2>&1; then
        break
    fi
    printf "."
    sleep 1
done
echo ""

# ─── 检查结果 ────────────────────────────────────────────────────────
if curl -s "http://localhost:${CDP_PORT}/json/version" > /dev/null 2>&1; then
    VERSION=$(curl -s "http://localhost:${CDP_PORT}/json/version" 2>/dev/null | grep -o '"Browser":"[^"]*"' | cut -d'"' -f4 || echo "未知")
    
    echo ""
    echo "✓ Chrome CDP 启动成功！"
    echo ""
    echo "  PID: $CHROME_PID"
    echo "  Version: $VERSION"
    echo "  CDP URL: http://localhost:${CDP_PORT}"
    echo "  WebSocket: http://localhost:${CDP_PORT}/devtools/browser/..."
    echo "  User Data: $USER_DATA_DIR"
    echo "  Log: $TMP_LOG"
    echo ""
    echo "=========================================="
    echo "使用方式："
    echo "=========================================="
    echo "1. 在浏览器中完成需要登录的页面"
    echo "2. 登录成功后，Agent 将自动提取信息"
    echo ""
    echo "Playwright 连接："
    echo "  chromium.connectOverCDP('http://localhost:${CDP_PORT}')"
    echo ""
    echo "停止命令："
    echo "  pkill -f 'remote-debugging-port=${CDP_PORT}'"
    echo "=========================================="
else
    echo ""
    echo "✗ Chrome CDP 启动失败"
    echo ""
    echo "检查："
    echo "  1. Chrome 路径: $CHROME_PATH"
    echo "  2. 端口 ${CDP_PORT} 是否被占用"
    echo "  3. 日志文件: $TMP_LOG"
    echo ""
    echo "手动启动："
    echo "  \"$CHROME_PATH\" --remote-debugging-port=${CDP_PORT} --user-data-dir=\"$USER_DATA_DIR\""
    exit 1
fi
