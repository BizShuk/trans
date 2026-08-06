package main

import (
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/bizshuk/trans"

// allowedImports 是 CLAUDE.md 分層規則的可執行版本。
//
// 文件裡寫得出 pass/fail 的斷言必須自動化 —— 沒人執行的斷言會腐爛成錯誤資訊。
// 新增套件時必須在這裡宣告它的層，否則 TestContractCoversEveryPackage 會失敗。
//
// `sdk` 套件是 SDK 本體。它`不得`依賴 config：任何 import 這個 SDK 的程式
// 都不該被迫接受一個全域設定來源。
var allowedImports = map[string][]string{
	".":                {"cli", "config"},
	"sdk":              {"engine", "engine/opencc", "engine/openaiapi", "engine/ct2", "glossary", "segment"},
	"segment":          {},
	"engine":           {},
	"engine/opencc":    {"engine"},
	"engine/openaiapi": {"engine"},
	"engine/ct2":       {"engine"},
	"glossary":         {},
	"benchmark":        {"sdk"},
	"config":           {},
	"cli":              {"sdk", "benchmark", "config", "glossary"},
}

func TestPackagesFollowDependencyContract(t *testing.T) {
	for pkg, allowed := range allowedImports {
		t.Run(pkg, func(t *testing.T) {
			imports, err := internalImports(pkg)
			if err != nil {
				t.Fatalf("scan %s: %v", pkg, err)
			}
			for _, imp := range imports {
				if !slices.Contains(allowed, imp) {
					t.Errorf("%s imports %s, which the layer contract does not allow\nallowed: %v",
						pkg, imp, allowed)
				}
			}
		})
	}
}

// SDK 是主體，設定是應用程式的事。這條最容易被順手違反，單獨釘住。
func TestSDKDoesNotDependOnConfig(t *testing.T) {
	for _, pkg := range []string{"sdk", "segment", "engine", "engine/opencc", "engine/openaiapi", "engine/ct2", "glossary"} {
		imports, err := internalImports(pkg)
		if err != nil {
			t.Fatalf("scan %s: %v", pkg, err)
		}
		if slices.Contains(imports, "config") {
			t.Errorf("%s imports config；SDK 不得綁定全域設定來源", pkg)
		}
	}
}

// 引擎彼此獨立：一個 adapter 依賴另一個 adapter，等於把兩者的部署需求綁在一起。
func TestEnginesDoNotDependOnEachOther(t *testing.T) {
	for pkg := range allowedImports {
		if !strings.HasPrefix(pkg, "engine/") {
			continue
		}
		imports, err := internalImports(pkg)
		if err != nil {
			t.Fatalf("scan %s: %v", pkg, err)
		}
		for _, imp := range imports {
			if strings.HasPrefix(imp, "engine/") && imp != pkg {
				t.Errorf("%s imports %s；引擎之間必須彼此獨立", pkg, imp)
			}
		}
	}
}

func TestContractCoversEveryPackage(t *testing.T) {
	for pkg := range allowedImports {
		if _, err := os.Stat(pkg); os.IsNotExist(err) {
			t.Errorf("contract names %s but that directory does not exist", pkg)
		}
	}
	for _, pkg := range walkPackages(t) {
		if _, declared := allowedImports[pkg]; !declared {
			t.Errorf("package %s exists but is not declared in the layer contract", pkg)
		}
	}
}

func TestRootPackageIsExecutable(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("scan module root: %v", err)
	}
	if pkg.Name != "main" {
		t.Errorf("module root package = %q, want main", pkg.Name)
	}
	if !slices.Contains(pkg.GoFiles, "main.go") {
		t.Errorf("module root Go files = %v, want main.go", pkg.GoFiles)
	}
	if _, err := os.Stat("cmd/translate"); !os.IsNotExist(err) {
		t.Errorf("obsolete cmd/translate still exists")
	}
}

func TestExecutableUsesTransAppName(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "translate")
	buildCmd := exec.Command("go", "build", "-o", binary, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build root executable: %v\n%s", err, output)
	}

	configBase := t.TempDir()
	runCmd := exec.Command(binary, "config", "--files")
	runCmd.Dir = t.TempDir()
	runCmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configBase)
	if output, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("run root executable: %v\n%s", err, output)
	}

	settings := filepath.Join(configBase, "trans", "settings.json")
	if _, err := os.Stat(settings); err != nil {
		t.Errorf("gosdk settings path = %s: %v", settings, err)
	}
}

func TestRuntimePathsDoNotUseObsoleteAppName(t *testing.T) {
	paths := []string{
		"cli/glossary.go",
		"cli/pull.go",
		"ecosystem.config.js",
		"scripts/ct2_server.py",
		"scripts/pull_model.py",
		"scripts/run.sh",
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(raw)
		if strings.Contains(content, ".config/translate") ||
			strings.Contains(content, `".config" / "translate"`) {
			t.Errorf("%s still uses obsolete translate config path", path)
		}
	}
}

// walkPackages 找出模組內所有含 .go 檔的套件目錄（不含 testdata）。
func walkPackages(t *testing.T) []string {
	t.Helper()

	var out []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") && path != "." || d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entries, _ := filepath.Glob(filepath.Join(path, "*.go")); len(entries) > 0 {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk packages: %v", err)
	}
	return out
}

// internalImports 回傳某套件對本模組其他套件的 import（含測試檔）。
func internalImports(pkg string) ([]string, error) {
	p, err := build.ImportDir(filepath.Clean(pkg), 0)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var out []string
	for _, imp := range append(append([]string{}, p.Imports...), p.TestImports...) {
		if imp == modulePath {
			imp = modulePath + "/." // 根套件在契約表裡以 "." 表示
		}
		rel, ok := strings.CutPrefix(imp, modulePath+"/")
		if !ok || seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
	}
	return out, nil
}
