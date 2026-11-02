// Command i18n lists TouchUp's messages and checks its translations.
//
//	go run ./tools/i18n          check every assets/locales/*.json
//	go run ./tools/i18n -write   also rewrite tools/i18n/template.json
//
// Messages are the English text passed to tr.T and tr.N in the Go code,
// and the descriptions, requirements, tags, model names and option text
// of the built-in engine catalog. template.json maps each to its English
// text (plural messages to their English forms), for translators. The
// check reports, per language, messages that are missing or no longer
// used, and translations whose {placeholders} differ from the English.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func main() {
	write := flag.Bool("write", false, "rewrite tools/i18n/template.json")
	flag.Parse()

	msgs := map[string]bool{} // message -> has plural forms
	var problems []string
	for _, root := range []string{"main.go", "internal"} {
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			problems = append(problems, scanGo(path, msgs)...)
			return nil
		})
	}
	catalog, err := filepath.Glob("internal/engine/catalog/*.json")
	if err != nil {
		fail(err)
	}
	for _, path := range catalog {
		problems = append(problems, scanCatalog(path, msgs)...)
	}

	english := readLocale("assets/locales/en.json")
	template := map[string]any{}
	for m, plural := range msgs {
		switch {
		case plural && english[m] == nil:
			problems = append(problems, fmt.Sprintf("en.json: plural message %q has no English forms", m))
		case plural:
			template[m] = english[m]
		default:
			template[m] = m
		}
	}
	if *write {
		writeJSON("tools/i18n/template.json", template)
	}

	files, _ := filepath.Glob("assets/locales/*.json")
	for _, path := range files {
		name := filepath.Base(path)
		if name == "en.json" {
			continue
		}
		problems = append(problems, check(name, readLocale(path), template)...)
	}
	fmt.Printf("%d messages, %d translations\n", len(msgs), len(files)-1)
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// scanGo collects the literal messages of tr.T and tr.N calls in path.
func scanGo(path string, msgs map[string]bool) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "tr" || (sel.Sel.Name != "T" && sel.Sel.Name != "N") {
			return true
		}
		s, ok := literal(call.Args[0])
		if !ok {
			// Text from the catalog or a custom manifest, collected from
			// the catalog itself.
			return true
		}
		msgs[s] = msgs[s] || sel.Sel.Name == "N"
		return true
	})
	return problems
}

// literal evaluates a string literal, or a concatenation of them.
func literal(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		a, ok1 := literal(e.X)
		b, ok2 := literal(e.Y)
		return a + b, ok1 && ok2
	case *ast.ParenExpr:
		return literal(e.X)
	}
	return "", false
}

// scanCatalog collects an engine manifest's translatable text.
func scanCatalog(path string, msgs map[string]bool) []string {
	var m struct {
		Description  string   `json:"description"`
		Requirements string   `json:"requirements"`
		Tags         []string `json:"tags"`
		Install      struct {
			Size string `json:"size"`
		} `json:"install"`
		Models []struct {
			Name, Description string
		} `json:"models"`
		Options []struct {
			Name, Description string
			Choices           []struct{ Label string }
		} `json:"options"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{err.Error()}
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return []string{path + ": " + err.Error()}
	}
	add := func(s string) {
		if words.MatchString(placeholder.ReplaceAllString(s, "")) {
			msgs[s] = msgs[s] || false
		}
	}
	if !bareSize.MatchString(m.Install.Size) {
		add(m.Install.Size)
	}
	add(m.Description)
	add(m.Requirements)
	for _, t := range m.Tags {
		add(t)
	}
	for _, md := range m.Models {
		add(md.Name)
		add(md.Description)
	}
	for _, o := range m.Options {
		add(o.Name)
		add(o.Description)
		for _, c := range o.Choices {
			add(c.Label)
		}
	}
	return nil
}

var placeholder = regexp.MustCompile(`\{[A-Za-z0-9_]+\}`)

// bareSize matches a size alone, "30 MB", which needs no translation.
var bareSize = regexp.MustCompile(`^≈?\s?[\d.]+\s?[KMG]B$`)

// words matches text worth translating: labels like "2 · 2 · 4" and
// "29" read the same in every language.
var words = regexp.MustCompile(`[A-Za-z%]`)

func placeholders(s string) []string {
	p := placeholder.FindAllString(s, -1)
	slices.Sort(p)
	return slices.Compact(p)
}

// check compares a translation with the template.
func check(name string, tr map[string]any, template map[string]any) []string {
	var problems []string
	keys := make([]string, 0, len(template))
	for k := range template {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	missing := 0
	for _, k := range keys {
		v, ok := tr[k]
		if !ok {
			missing++
			continue
		}
		want := placeholders(k)
		texts := []string{}
		switch v := v.(type) {
		case string:
			texts = append(texts, v)
		case map[string]any:
			if _, ok := template[k].(map[string]any); !ok {
				problems = append(problems, fmt.Sprintf("%s: %q has plural forms but isn't a plural message", name, k))
			}
			if _, ok := v["other"]; !ok {
				problems = append(problems, fmt.Sprintf("%s: %q has no \"other\" form", name, k))
			}
			for form, s := range v {
				if !slices.Contains([]string{"zero", "one", "two", "few", "many", "other"}, form) {
					problems = append(problems, fmt.Sprintf("%s: %q has an unknown plural form %q", name, k, form))
				}
				if s, ok := s.(string); ok {
					texts = append(texts, s)
				}
			}
			// Forms may leave the count out ("one" as "a file"), so check
			// the other placeholders only.
			want = slices.DeleteFunc(want, func(p string) bool { return p == "{count}" })
		}
		for _, s := range texts {
			got := slices.DeleteFunc(placeholders(s), func(p string) bool {
				_, plural := v.(map[string]any)
				return plural && p == "{count}"
			})
			if !slices.Equal(got, want) {
				problems = append(problems, fmt.Sprintf("%s: %q has placeholders %v, want %v: %q", name, k, got, want, s))
			}
		}
	}
	if missing > 0 {
		problems = append(problems, fmt.Sprintf("%s: %d of %d messages missing", name, missing, len(template)))
	}
	for k := range tr {
		if _, ok := template[k]; !ok {
			problems = append(problems, fmt.Sprintf("%s: %q is no longer used", name, k))
		}
	}
	return problems
}

func readLocale(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}
		}
		fail(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		fail(fmt.Errorf("%s: %w", path, err))
	}
	return m
}

func writeJSON(path string, v any) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
