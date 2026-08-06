#!/usr/bin/env bash
# 啟動 CTranslate2 sidecar。
#
# 相依（ctranslate2、transformers、fastapi）裝在使用者的共用虛擬環境裡。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 解析順序與 CLI 一致：已啟用的環境 → 共用 ~/.venv → 專案自帶 .venv。
# 兩個都裝著 ctranslate2 的環境並存，遲早會出現「命令列能跑、sidecar 不能跑」。
PYTHON=""
for candidate in "${VIRTUAL_ENV:-}/bin/python" "${HOME}/.venv/bin/python" "${ROOT}/.venv/bin/python"; do
  if [[ -x "${candidate}" ]]; then
    PYTHON="${candidate}"
    break
  fi
done

if [[ -z "${PYTHON}" ]]; then
  echo "找不到可用的 python；請在虛擬環境中安裝相依：" >&2
  echo "  uv pip install ctranslate2 transformers sentencepiece fastapi uvicorn huggingface_hub" >&2
  exit 1
fi

exec "${PYTHON}" "${ROOT}/scripts/ct2_server.py"
