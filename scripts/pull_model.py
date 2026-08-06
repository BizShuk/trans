#!/usr/bin/env python3
"""下載並安裝 CTranslate2 格式的翻譯模型。

優先抓`已轉換好`的 CTranslate2 版本，而不是抓 HuggingFace 原始權重再轉：
轉換需要 PyTorch（數百 MB 的相依），只為了做一次一次性的格式轉換。

用法：
    pull_model.py nllb-600m
    pull_model.py opus-mt-en-zh
    pull_model.py --list
"""

from __future__ import annotations

import argparse
import os
import shutil
import sys
from pathlib import Path

from huggingface_hub import snapshot_download

MODELS_DIR = Path(
    os.environ.get("TRANSLATE_MODELS_DIR", Path.home() / ".config" / "trans" / "models")
)

# 每個項目是「本機名稱 → 來源 repo」，名稱與 Go 端的 Model 常數對齊，
# 使用者不必記兩套名字。
#
# mode 決定取得方式：
#   download  已經是 CTranslate2 格式，直接抓
#   convert   HuggingFace 原始權重，本地轉一次（需要 torch）
#
# 實測 2026-08：NLLB-200-distilled-600M 有社群轉好的版本，
# OPUS-MT 沒有 —— 因此後者只能走轉換路徑。
CATALOG = {
    "nllb-600m": {
        "repo": "entai2965/nllb-200-distilled-600M-ctranslate2",
        "mode": "download",
        "size": "~1.2 GB",
        "note": "Meta NLLB-200-distilled-600M，200 語互譯",
    },
    "opus-mt-en-zh": {
        "repo": "Helsinki-NLP/opus-mt-en-zh",
        "mode": "convert",
        "size": "~300 MB",
        "note": "Helsinki-NLP OPUS-MT，英 → 中（本地轉換，需 torch）",
    },
    "opus-mt-zh-en": {
        "repo": "Helsinki-NLP/opus-mt-zh-en",
        "mode": "convert",
        "size": "~300 MB",
        "note": "Helsinki-NLP OPUS-MT，中 → 英（本地轉換，需 torch）",
    },
}


def list_models() -> None:
    installed = {p.name for p in MODELS_DIR.iterdir()} if MODELS_DIR.exists() else set()
    print(f"模型目錄：{MODELS_DIR}\n")
    print(f"{'NAME':<18}{'SIZE':<12}{'STATUS':<12}NOTE")
    for name, spec in CATALOG.items():
        status = "installed" if name in installed else "-"
        print(f"{name:<18}{spec['size']:<12}{status:<12}{spec['note']}")


def pull(name: str) -> int:
    spec = CATALOG.get(name)
    if spec is None:
        print(f"未知模型 {name}；可用：{', '.join(CATALOG)}", file=sys.stderr)
        return 1

    target = MODELS_DIR / name
    if (target / "model.bin").exists():
        print(f"{name} 已安裝於 {target}")
        return 0

    MODELS_DIR.mkdir(parents=True, exist_ok=True)

    # 先落在暫存目錄再改名：中途被砍時不會留下一個看起來完整的模型目錄，
    # 而半份模型的失敗訊息會出現在載入時，離問題發生已經很遠。
    staging = MODELS_DIR / f".{name}.partial"

    # 暫存目錄已存在 = 另一個下載可能還在跑。實測過兩個行程共用同一個
    # 暫存目錄的下場：其中一個的清理刪掉另一個正在寫的檔案，
    # 兩邊都以「檔案不存在」失敗，而錯誤訊息完全看不出真正的原因。
    if staging.exists():
        print(
            f"{staging} 已存在，另一個下載可能正在進行。\n"
            f"確定沒有的話請先刪除：rm -rf {staging}",
            file=sys.stderr,
        )
        return 1

    try:
        if spec["mode"] == "download":
            print(f"下載 {spec['repo']}（{spec['size']}）…", file=sys.stderr)
            # 連線數壓到 2：這條網路上多開反而讓每條都慢到逾時。
            snapshot_download(repo_id=spec["repo"], local_dir=str(staging), max_workers=2)
        else:
            print(f"下載並轉換 {spec['repo']}（{spec['size']}）…", file=sys.stderr)
            convert(spec["repo"], staging)

        if not (staging / "model.bin").exists():
            print(f"{spec['repo']} 沒有產出 model.bin，不是 CTranslate2 格式", file=sys.stderr)
            return 1
        staging.rename(target)
    except Exception as exc:  # noqa: BLE001
        shutil.rmtree(staging, ignore_errors=True)
        print(f"取得失敗：{exc}", file=sys.stderr)
        return 1

    print(f"完成：{target}")
    return 0


def convert(repo: str, target: Path) -> None:
    """把 HuggingFace 原始權重轉成 CTranslate2 格式。

    轉換要載入原始權重，因此需要 torch。它只在這條路徑上需要 ——
    直接抓已轉好的模型不必裝，所以不列為必要相依。
    """
    try:
        import torch  # noqa: F401
    except ImportError as exc:
        raise RuntimeError(
            "轉換需要 PyTorch：uv pip install --python .venv/bin/python torch"
        ) from exc

    from ctranslate2.converters import TransformersConverter

    converter = TransformersConverter(repo, copy_files=["tokenizer_config.json"])
    # int8：M4 上 CTranslate2 只有 CPU 路徑，量化是這個平台唯一划算的選擇。
    converter.convert(str(target), quantization="int8", force=True)

    # tokenizer 要與模型同目錄，sidecar 才能用 AutoTokenizer 直接載入。
    from transformers import AutoTokenizer

    AutoTokenizer.from_pretrained(repo).save_pretrained(str(target))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("model", nargs="?", help="要下載的模型名稱")
    parser.add_argument("--list", action="store_true", help="列出可下載的模型")
    args = parser.parse_args()

    if args.list or not args.model:
        list_models()
        return 0
    return pull(args.model)


if __name__ == "__main__":
    raise SystemExit(main())
