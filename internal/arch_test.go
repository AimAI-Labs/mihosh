package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/AimAI-Labs/mihosh/"

func TestArchitectureBoundaries(t *testing.T) {
	pkgImports := collectInternalImports(t)

	var violations []string

	pkgs := make([]string, 0, len(pkgImports))
	for pkg := range pkgImports {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		imports := pkgImports[pkg]
		for _, imp := range imports {
			switch {
			case hasLayerPrefix(pkg, "internal/domain"):
				if hasAnyLayerPrefix(imp, "internal/app", "internal/infrastructure", "internal/ui", "internal/cli", "internal/bootstrap") {
					violations = append(violations, pkg+" -> "+imp+" (domain must remain dependency-free from outer layers)")
				}

			case hasLayerPrefix(pkg, "internal/app"):
				if hasAnyLayerPrefix(imp, "internal/ui", "internal/cli", "internal/bootstrap") {
					violations = append(violations, pkg+" -> "+imp+" (app layer must not depend on ui, cli, or bootstrap)")
				}

			case hasLayerPrefix(pkg, "internal/infrastructure"):
				if hasAnyLayerPrefix(imp, "internal/app", "internal/ui", "internal/cli", "internal/bootstrap") {
					violations = append(violations, pkg+" -> "+imp+" (infrastructure must not depend on app, ui, cli, or bootstrap)")
				}

			case hasLayerPrefix(pkg, "internal/bootstrap"):
				if hasAnyLayerPrefix(imp, "internal/ui", "internal/cli") {
					violations = append(violations, pkg+" -> "+imp+" (bootstrap must not depend on ui or cli)")
				}

			case hasLayerPrefix(pkg, "internal/ui"):
				if hasLayerPrefix(imp, "internal/cli") {
					violations = append(violations, pkg+" -> "+imp+" (ui must not depend on cli)")
				}
				if hasLayerPrefix(imp, "internal/infrastructure") {
					violations = append(violations, pkg+" -> "+imp+" (ui must depend on internal/app/service or internal/domain contracts)")
				}
			}
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("architecture boundary violations detected:\n- %s", strings.Join(violations, "\n- "))
	}
}

func collectInternalImports(t *testing.T) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	seen := make(map[string]map[string]bool)

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(path))
		pkg := "internal"
		if dir != "." {
			pkg = "internal/" + dir
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if seen[pkg] == nil {
			seen[pkg] = make(map[string]bool)
		}
		for _, spec := range file.Imports {
			raw, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			if !strings.HasPrefix(raw, modulePrefix) {
				continue
			}
			seen[pkg][strings.TrimPrefix(raw, modulePrefix)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed scanning internal imports: %v", err)
	}

	result := make(map[string][]string, len(seen))
	for pkg, set := range seen {
		list := make([]string, 0, len(set))
		for imp := range set {
			list = append(list, imp)
		}
		sort.Strings(list)
		result[pkg] = list
	}
	return result
}

func hasLayerPrefix(pkg, layer string) bool {
	return pkg == layer || strings.HasPrefix(pkg, layer+"/")
}

func hasAnyLayerPrefix(pkg string, layers ...string) bool {
	for _, layer := range layers {
		if hasLayerPrefix(pkg, layer) {
			return true
		}
	}
	return false
}
