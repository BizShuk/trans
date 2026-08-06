#!/usr/bin/env python3
"""CTranslate2 sidecar：把 encoder-decoder 翻譯模型包成一個最小的 HTTP 服務。

它存在的唯一理由是 CTranslate2 只有 Python／C++ 綁定，而 NLLB-200 與
OPUS-MT 都是 encoder-decoder 架構，llama.cpp 不支援。契約由本專案定義：

    GET  /health              -> {"models": ["nllb-600m", ...]}
    POST /translate           -> {"text": "...", "model": "..."}
         {"text", "target", "source"?, "model"?}

不用 LibreTranslate 的理由見 CLAUDE.md：它自帶 web UI、API key 管理與
argos 套件系統，而我們要的只是「送一段字、回一段字」。

模型放在 ~/.config/trans/models/<name>/，由 scripts/pull_model.py 下載。
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

import ctranslate2
import transformers
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

MODELS_DIR = Path(
    os.environ.get("TRANSLATE_MODELS_DIR", Path.home() / ".config" / "trans" / "models")
)

# NLLB 用自己的語言碼（FLORES-200），與 BCP-47 對不上，因此要一張對照表。
# 少了它，使用者得先知道台灣繁體在 NLLB 裡叫 zho_Hant 才能翻一句話。
NLLB_LANGS = {
    "zh": "zho_Hans",
    "zh-cn": "zho_Hans",
    "zh-hans": "zho_Hans",
    "zh-tw": "zho_Hant",
    "zh-hant": "zho_Hant",
    "zh-hk": "yue_Hant",
    "en": "eng_Latn",
    "ja": "jpn_Jpan",
    "ko": "kor_Hang",
    "de": "deu_Latn",
    "fr": "fra_Latn",
    "es": "spa_Latn",
    "th": "tha_Thai",
    "vi": "vie_Latn",
    "id": "ind_Latn",
}


class Model:
    """一顆已載入的模型：CTranslate2 translator 加上它的 tokenizer。"""

    def __init__(self, name: str, path: Path) -> None:
        self.name = name
        self.path = path
        # M4 上 CTranslate2 只有 CPU 路徑（沒有 Metal 後端），
        # int8 量化是這個平台上唯一划算的選擇。
        self.translator = ctranslate2.Translator(str(path), device="cpu", compute_type="int8")
        self.tokenizer = transformers.AutoTokenizer.from_pretrained(str(path))
        self.is_nllb = "nllb" in name.lower()

    def translate(self, text: str, source: str, target: str) -> str:
        if self.is_nllb:
            return self._translate_nllb(text, source, target)
        return self._translate_plain(text)

    def _translate_nllb(self, text: str, source: str, target: str) -> str:
        src = flores_code(source) if source else None
        tgt = flores_code(target)
        if tgt is None:
            raise HTTPException(400, f"NLLB 沒有 {target} 的語言碼；可用：{sorted(NLLB_LANGS)}")

        if src:
            self.tokenizer.src_lang = src
        tokens = self.tokenizer.convert_ids_to_tokens(self.tokenizer.encode(text))
        results = self.translator.translate_batch(
            [tokens],
            target_prefix=[[tgt]],
            beam_size=4,
            max_decoding_length=512,
        )
        out = results[0].hypotheses[0]
        # 第一個 token 是我們自己塞的目標語言標記，不屬於譯文。
        if out and out[0] == tgt:
            out = out[1:]
        return detokenize(self.tokenizer, out)

    def _translate_plain(self, text: str) -> str:
        tokens = self.tokenizer.convert_ids_to_tokens(self.tokenizer.encode(text))
        results = self.translator.translate_batch([tokens], beam_size=4, max_decoding_length=512)
        return detokenize(self.tokenizer, results[0].hypotheses[0])


def detokenize(tokenizer, tokens: list[str]) -> str:
    """把 CTranslate2 回傳的 token 串接回文字。

    `convert_tokens_to_ids` → `decode` 的往返會把 CT2 詞彙表裡有、
    但 HF tokenizer 對不上的 token 變成 `<unk>`（實測：`毛利率` → `<unk>利率`）。
    `convert_tokens_to_string` 直接走 sentencepiece 的還原，不經過 id。
    """
    text = tokenizer.convert_tokens_to_string(
        [t for t in tokens if t not in tokenizer.all_special_tokens]
    )
    return text.strip()


def flores_code(tag: str) -> str | None:
    if not tag:
        return None
    key = tag.strip().lower().replace("_", "-")
    if key in NLLB_LANGS:
        return NLLB_LANGS[key]
    # zh-Hant-TW 這類長標籤退到前兩段再試一次。
    parts = key.split("-")
    for candidate in ("-".join(parts[:2]), parts[0]):
        if candidate in NLLB_LANGS:
            return NLLB_LANGS[candidate]
    return None


def available() -> list[str]:
    if not MODELS_DIR.exists():
        return []
    return sorted(p.name for p in MODELS_DIR.iterdir() if (p / "model.bin").exists())


class Loader:
    """依需求載入模型並保留在記憶體。

    啟動時不預載：一顆 NLLB-600M 佔約 1.2 GB，開機就吃掉它，
    只為了服務一個可能永遠不會來的請求。
    """

    def __init__(self) -> None:
        self._cache: dict[str, Model] = {}

    def get(self, name: str) -> Model:
        if name in self._cache:
            return self._cache[name]

        path = MODELS_DIR / name
        if not (path / "model.bin").exists():
            raise HTTPException(
                404,
                f"模型 {name} 未安裝；請執行 translate models pull {name}"
                f"（現有：{available() or '無'}）",
            )
        self._cache[name] = Model(name, path)
        return self._cache[name]


app = FastAPI(title="translate ct2 sidecar")
loader = Loader()


class TranslateRequest(BaseModel):
    text: str
    target: str
    source: str | None = None
    model: str = "nllb-600m"


@app.get("/health")
def health() -> dict:
    return {"models": available(), "models_dir": str(MODELS_DIR)}


@app.post("/translate")
def translate(req: TranslateRequest) -> dict:
    if not req.text.strip():
        return {"text": req.text, "model": req.model}

    model = loader.get(req.model)
    try:
        text = model.translate(req.text, req.source or "", req.target)
    except HTTPException:
        raise
    except Exception as exc:  # noqa: BLE001 - 轉成 HTTP 錯誤交給呼叫端判讀
        raise HTTPException(500, f"{req.model} 翻譯失敗：{exc}") from exc
    return {"text": text, "model": req.model}


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("TRANSLATE_CT2_PORT", "8765"))
    print(f"ct2 sidecar on :{port}，模型目錄 {MODELS_DIR}", file=sys.stderr)
    uvicorn.run(app, host="127.0.0.1", port=port, log_level="warning")
