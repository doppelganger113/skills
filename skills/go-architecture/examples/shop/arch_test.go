// Architecture fitness tests: the rules from the go-architecture skill
// that golangci-lint can't express, checked on every go test run. They
// use only the standard library, so they run anywhere the code builds.
//
// When a rule is wrong for this codebase, change it here in the same PR
// and say why; don't skip it.
package shop

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// bannedPackageNames say nothing about what a package provides.
var bannedPackageNames = map[string]bool{
	"base": true, "common": true, "helper": true, "helpers": true, "misc": true,
	"model": true, "models": true, "shared": true, "types": true, "util": true, "utils": true,
}

type goFile struct {
	path string // slash-separated, relative to the module root
	dir  string
	ast  *ast.File
}

// loadFiles parses every hand-written, non-test Go file under cmd/ and
// internal/. Generated code (sqlc, oapi-codegen) follows its generator's
// conventions, not ours, so it is skipped.
func loadFiles(t *testing.T) (module string, files []goFile) {
	t.Helper()
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(mod), "\n") {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			module = strings.TrimSpace(rest)
		}
	}
	fset := token.NewFileSet()
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution|parser.ParseComments)
			if err != nil {
				return err
			}
			if ast.IsGenerated(f) {
				return nil
			}
			p = filepath.ToSlash(p)
			files = append(files, goFile{path: p, dir: path.Dir(p), ast: f})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if len(files) == 0 {
		t.Fatal("no Go files found; run from the module root")
	}
	return module, files
}

// domainOf returns the domain a directory belongs to ("order" for
// internal/order and internal/order/internal/db), or "" for cmd/ and
// internal/platform/.
func domainOf(dir string) string {
	parts := strings.Split(dir, "/")
	if len(parts) < 2 || parts[0] != "internal" || parts[1] == "platform" {
		return ""
	}
	return parts[1]
}

func TestPackageNamesSayWhatTheyProvide(t *testing.T) {
	t.Parallel()
	_, files := loadFiles(t)
	for _, f := range files {
		if name := f.ast.Name.Name; bannedPackageNames[name] {
			t.Errorf("%s: package %q says nothing about what it provides; name it for its contents", f.path, name)
		}
	}
}

func TestDomainsAreNotNested(t *testing.T) {
	t.Parallel()
	_, files := loadFiles(t)
	for _, f := range files {
		parts := strings.Split(f.dir, "/")
		if domainOf(f.dir) != "" && len(parts) > 2 && parts[2] != "internal" {
			t.Errorf("%s: %s is nested inside domain %s; make it a sibling under internal/ or move it under %s/internal/",
				f.path, f.dir, parts[1], parts[1])
		}
	}
}

func TestNoInitFunctions(t *testing.T) {
	t.Parallel()
	_, files := loadFiles(t)
	for _, f := range files {
		for _, decl := range f.ast.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "init" {
				t.Errorf("%s: init() hides side effects; do the work in a constructor called from main", f.path)
			}
		}
	}
}

// Package-level vars are global state, with three immutable exceptions:
// sentinel errors (Err*, err*), //go:embed file systems, and values
// built once by a Must* call (regexp.MustCompile, template.Must).
func TestNoPackageLevelState(t *testing.T) {
	t.Parallel()
	_, files := loadFiles(t)
	for _, f := range files {
		for _, decl := range f.ast.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if hasEmbedDirective(gen.Doc) || hasEmbedDirective(vs.Doc) || allMustCalls(vs.Values) {
					continue
				}
				for _, name := range vs.Names {
					n := name.Name
					if n == "_" || strings.HasPrefix(n, "Err") || strings.HasPrefix(n, "err") {
						continue // sentinel errors and compile-time assertions
					}
					t.Errorf("%s: package-level var %s is global state; pass it through a constructor from main", f.path, n)
				}
			}
		}
	}
}

func hasEmbedDirective(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		if strings.HasPrefix(c.Text, "//go:embed ") {
			return true
		}
	}
	return false
}

func allMustCalls(values []ast.Expr) bool {
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		call, ok := v.(*ast.CallExpr)
		if !ok {
			return false
		}
		var name string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if !strings.HasPrefix(name, "Must") {
			return false
		}
	}
	return true
}

// Only store files run SQL. Services sit in the same package as the
// store, so the compiler can't stop them reaching its connection; this
// test flags it two ways, independent of driver or code generator:
//   - reaching through a store into its unexported fields (s.store.q),
//     which covers pgx and sqlc-generated methods too;
//   - calling database/sql or pgx query methods directly.
func TestOnlyStoresRunSQL(t *testing.T) {
	t.Parallel()
	_, files := loadFiles(t)
	dbCalls := map[string]bool{
		"ExecContext": true, "QueryContext": true, "QueryRowContext": true,
		"PrepareContext": true, "BeginTx": true, "Q": true,
	}
	pgxCalls := map[string]bool{"Exec": true, "Query": true, "QueryRow": true, "Begin": true}
	for _, f := range files {
		base := path.Base(f.path)
		if domainOf(f.dir) == "" || strings.HasPrefix(base, "store") || strings.Contains(f.dir, "/internal/") {
			continue
		}
		isHTTP := strings.HasPrefix(base, "http") // r.URL.Query() is legitimate there
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if inner, ok := sel.X.(*ast.SelectorExpr); ok &&
					strings.EqualFold(inner.Sel.Name, "store") && !ast.IsExported(sel.Sel.Name) {
					t.Errorf("%s: reaches into the store's field %s; add a Store method instead", f.path, sel.Sel.Name)
				}
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				name := sel.Sel.Name
				if dbCalls[name] || (pgxCalls[name] && !isHTTP) {
					t.Errorf("%s: calls %s; SQL belongs in store*.go (use store.InTx for atomic writes)", f.path, name)
				}
			}
			return true
		})
	}
}

func TestNoStutter(t *testing.T) {
	t.Parallel()
	_, files := loadFiles(t)
	for _, f := range files {
		pkg := f.ast.Name.Name
		if pkg == "main" {
			continue
		}
		check := func(name string) {
			if !ast.IsExported(name) || len(name) <= len(pkg) {
				return
			}
			if strings.EqualFold(name[:len(pkg)], pkg) && unicode.IsUpper(rune(name[len(pkg)])) {
				t.Errorf("%s: %s.%s stutters; call it %s.%s", f.path, pkg, name, pkg, name[len(pkg):])
			}
		}
		for _, decl := range f.ast.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					check(d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						check(ts.Name.Name)
					}
				}
			}
		}
	}
}

// Domains refer to each other's data by ID. A struct field typed with
// another domain's type couples their data models; the one allowed case
// is a dependency on the other domain's *Service.
func TestDomainsReferToEachOtherByID(t *testing.T) {
	t.Parallel()
	module, files := loadFiles(t)
	for _, f := range files {
		own := domainOf(f.dir)
		if own == "" {
			continue
		}
		otherDomain := map[string]string{} // import name to domain
		for _, imp := range f.ast.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			rel, ok := strings.CutPrefix(p, module+"/")
			if !ok {
				continue
			}
			d := domainOf(rel)
			if d == "" || d == own {
				continue
			}
			name := path.Base(p)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			otherDomain[name] = d
		}
		if len(otherDomain) == 0 {
			continue
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				ast.Inspect(field.Type, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					x, ok := sel.X.(*ast.Ident)
					if !ok {
						return true
					}
					if d, ok := otherDomain[x.Name]; ok && sel.Sel.Name != "Service" {
						t.Errorf("%s: struct field uses %s.%s from domain %s; store its ID instead",
							f.path, x.Name, sel.Sel.Name, d)
					}
					return true
				})
			}
			return true
		})
	}
}
