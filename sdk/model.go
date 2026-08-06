package translate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bizshuk/trans/engine/ct2"
	"github.com/bizshuk/trans/engine/openaiapi"
	"github.com/bizshuk/trans/engine/opencc"
)

// Model 是一個具名的轉換模型。
//
// 它與 Engine 分開：Engine 是`怎麼跑`（行程內字典、sidecar、本機模型服務），
// Model 是`跑哪一顆`。同一個 engine 可以承載多顆模型，
// 而使用者在意的是後者 —— 沒有人會說「我要用 ct2」，只會說「我要用 NLLB」。
type Model string

const (
	// ModelOpenCC 是中文字形與地區用語轉換，決定性、毫秒級、無需下載。
	ModelOpenCC Model = "opencc"

	// ModelNLLB600M 是 Meta NLLB-200-distilled-600M，一顆模型覆蓋 200 語。
	ModelNLLB600M Model = "nllb-600m"

	// ModelOpusMT 是 Helsinki-NLP OPUS-MT，單一語言對、最省資源。
	ModelOpusMT Model = "opus-mt"

	// ModelQwen25 是 Qwen2.5-1.5B-Instruct，中文語感佳、可下術語指示。
	ModelQwen25 Model = "qwen2.5-1.5b"
)

// Spec 描述一顆模型跑在哪個引擎上，以及它的預設連線參數。
type Spec struct {
	Model Model

	// Engine 是承載這顆模型的引擎名稱。
	Engine string

	// EngineModel 是引擎端的模型識別字。它與 Model 不同：
	// 我們叫它 qwen2.5-1.5b，ollama 叫它 qwen2.5:1.5b。
	EngineModel string

	// Requires 是這顆模型的執行期前提，空字串代表沒有。
	// 列出來是為了讓 Health 失敗時，使用者知道該去啟動什麼。
	Requires string

	// SentenceLevel 為真代表這顆模型一次只吃一個句子。
	// 專用 NMT 都是這樣訓練的，多餵會安靜地漏掉整句。
	SentenceLevel bool

	// NormalizeScript 為真代表這顆模型的中文字形不可靠，
	// 目標是繁體時要補一次 OpenCC 轉換。
	//
	// 實測：opus-mt-en-zh 固定出简體；NLLB 即使指定 zho_Hant 仍會漏字
	// （`管理层提升了全年指引`）。對已經是繁體的文字再跑一次 s2twp 近乎恆等，
	// 因此補這一刀是安全的。
	NormalizeScript bool

	Description string
}

var specs = map[Model]Spec{
	ModelOpenCC: {
		Model: ModelOpenCC, Engine: opencc.Name,
		Description: "OpenCC 字典，中文字形與地區用語，行程內、決定性",
	},
	ModelQwen25: {
		Model: ModelQwen25, Engine: openaiapi.Name, EngineModel: "qwen2.5:1.5b",
		Requires:    "ollama 或 llama-server",
		Description: "Qwen2.5-1.5B-Instruct，中文語感佳、可下術語指示",
	},
	ModelNLLB600M: {
		Model: ModelNLLB600M, Engine: ct2.Name, EngineModel: "nllb-600m",
		Requires:        "CTranslate2 sidecar",
		SentenceLevel:   true,
		NormalizeScript: true,
		Description:     "NLLB-200-distilled-600M，200 語互譯",
	},
	ModelOpusMT: {
		Model: ModelOpusMT, Engine: ct2.Name, EngineModel: "opus-mt-en-zh",
		Requires:        "CTranslate2 sidecar",
		SentenceLevel:   true,
		NormalizeScript: true,
		// OPUS-MT 是`每個語言對一顆模型`，因此預設只綁英→中；
		// 其他方向以 EngineModel（CLI 的 --engine-model）指定。
		Description: "Helsinki-NLP OPUS-MT，單一語言對、最省資源",
	},
}

// Models 回傳所有已知模型，依名稱排序。
func Models() []Spec {
	out := make([]Spec, 0, len(specs))
	for _, s := range specs {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// Lookup 取得模型規格。
func Lookup(m Model) (Spec, error) {
	spec, ok := specs[m]
	if !ok {
		return Spec{}, fmt.Errorf("translate: 未知模型 %q（可用：%s）", m, modelNames())
	}
	return spec, nil
}

// route 依語言對挑預設模型。
//
// 中文互轉一律走 OpenCC —— 那是查表問題，交給神經模型只會換來不確定性
// 與百倍延遲。其餘語言對走 NLLB：一顆模型覆蓋所有語言對，
// 不必為每個方向各裝一顆。
func route(from, to string) Model {
	if opencc.New().Supports(from, to) {
		return ModelOpenCC
	}
	return ModelNLLB600M
}

func modelNames() string {
	names := make([]string, 0, len(specs))
	for m := range specs {
		names = append(names, string(m))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
