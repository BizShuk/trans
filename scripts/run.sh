#!/usr/bin/env bash
set -euo pipefail

# 建立預設資料與日誌目錄
mkdir -p "${HOME}/.config/trans/models"
mkdir -p "${HOME}/.config/trans/logs"

chmod +x "$(dirname "$0")"/*.sh 2>/dev/null || true
echo "trans metadata setup complete."
