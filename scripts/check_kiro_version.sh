#!/bin/bash
# 自动检测 Kiro IDE 版本信息
# 用法: ./scripts/check_kiro_version.sh

KIRO_PATH="/usr/share/kiro/resources/app"

if [ ! -d "$KIRO_PATH" ]; then
    echo "错误: Kiro IDE 未安装在 $KIRO_PATH"
    exit 1
fi

echo "=== Kiro IDE 版本信息 ==="
echo ""

# 主版本
IDE_VERSION=$(cat "$KIRO_PATH/package.json" | grep -o '"version": "[^"]*"' | head -1 | cut -d'"' -f4)
echo "IDE 版本: $IDE_VERSION"

# Node.js 版本 (从 kiro-agent 扩展)
NODE_VERSION=$(cat "$KIRO_PATH/extensions/kiro.kiro-agent/package.json" 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('engines',{}).get('node',''))" 2>/dev/null)
echo "Node 版本: $NODE_VERSION"

# SDK 版本 (从 codewhisperer-streaming-client)
SDK_VERSION=$(grep -A5 'codewhisperer-streaming-client/package.json' "$KIRO_PATH/extensions/kiro.kiro-agent/dist/extension.js" 2>/dev/null | grep -oE 'version: "[0-9]+\.[0-9]+\.[0-9]+"' | head -1 | cut -d'"' -f2)
echo "SDK 版本: $SDK_VERSION"

# 当前配置
echo ""
echo "=== 当前 config/config.go 配置 ==="
grep -E "KiroIDEVersion|KiroNodeVersion|KiroSDKVersion" config/config.go 2>/dev/null | head -5

echo ""
echo "=== 对比 ==="
CURRENT_IDE=$(grep "KiroIDEVersion" config/config.go 2>/dev/null | grep -o '"[^"]*"' | tr -d '"')
CURRENT_NODE=$(grep "KiroNodeVersion" config/config.go 2>/dev/null | grep -o '"[^"]*"' | tr -d '"')
CURRENT_SDK=$(grep "KiroSDKVersion" config/config.go 2>/dev/null | grep -o '"[^"]*"' | tr -d '"')

if [ "$IDE_VERSION" != "$CURRENT_IDE" ]; then
    echo "⚠️  IDE 版本需要更新: $CURRENT_IDE -> $IDE_VERSION"
else
    echo "✓ IDE 版本一致"
fi

if [ "$NODE_VERSION" != "$CURRENT_NODE" ]; then
    echo "⚠️  Node 版本需要更新: $CURRENT_NODE -> $NODE_VERSION"
else
    echo "✓ Node 版本一致"
fi

if [ -n "$SDK_VERSION" ] && [ "$SDK_VERSION" != "$CURRENT_SDK" ]; then
    echo "⚠️  SDK 版本需要更新: $CURRENT_SDK -> $SDK_VERSION"
elif [ -n "$SDK_VERSION" ]; then
    echo "✓ SDK 版本一致"
else
    echo "⚠️  无法检测 SDK 版本"
fi

echo ""
echo "更新命令:"
echo "sed -i 's/KiroIDEVersion  = \"[^\"]*\"/KiroIDEVersion  = \"$IDE_VERSION\"/' config/config.go"
echo "sed -i 's/KiroNodeVersion = \"[^\"]*\"/KiroNodeVersion = \"$NODE_VERSION\"/' config/config.go"
[ -n "$SDK_VERSION" ] && echo "sed -i 's/KiroSDKVersion  = \"[^\"]*\"/KiroSDKVersion  = \"$SDK_VERSION\"/' config/config.go"
