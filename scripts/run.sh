#!/usr/bin/env bash
set -euo pipefail

# 建立預設資料與日誌目錄。
#
# `不`建立 models/：NMT 權重屬於 inf 的 ct2 元件（~/.cache/ct2/models），
# 本專案只是它的客戶端，在這裡先建一個空目錄只會讓人以為模型該放這邊。
mkdir -p "${HOME}/.config/trans/logs"

chmod +x "$(dirname "$0")"/*.sh 2>/dev/null || true
echo "trans metadata setup complete."
