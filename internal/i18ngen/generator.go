package i18ngen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type sourceMessage struct {
	Action string
	Code   int
	Msg    string
	File   string
	Line   int
}

type sourceScan struct {
	Messages map[string]sourceMessage
}

func Generate(root, dataPath, language string) (int, string, bool, error) {
	scan, err := scanSource(root)
	if err != nil {
		return 0, "", false, err
	}
	language = normalizeLanguage(language)
	if language == "" {
		return 0, "", false, fmt.Errorf("invalid language")
	}
	if !filepath.IsAbs(dataPath) {
		dataPath = filepath.Join(root, dataPath)
	}
	output := filepath.Join(dataPath, "i18n", language+".yaml")

	messages := make(map[string]string)
	if data, err := os.ReadFile(output); err == nil && len(data) > 0 {
		if err := yaml.Unmarshal(data, &messages); err != nil {
			return 0, output, false, fmt.Errorf("parse %s: %w", output, err)
		}
	}
	for key, item := range scan.Messages {
		if _, exists := messages[key]; !exists {
			messages[key] = item.Msg
		}
	}

	data, err := marshalCatalog(messages)
	if err != nil {
		return 0, output, false, err
	}
	changed, err := writeFileIfChanged(output, data)
	if err != nil {
		return 0, output, false, err
	}
	return len(scan.Messages), output, changed, nil
}

func Check(root, dataPath, language string) ([]string, error) {
	scan, err := scanSource(root)
	if err != nil {
		return nil, err
	}
	language = normalizeLanguage(language)
	if language == "" {
		return nil, fmt.Errorf("invalid language")
	}
	if !filepath.IsAbs(dataPath) {
		dataPath = filepath.Join(root, dataPath)
	}
	dir := filepath.Join(dataPath, "i18n")
	defaultMessages, err := readCatalog(filepath.Join(dir, language+".yaml"))
	if err != nil {
		return nil, err
	}

	var warnings []string
	for key, item := range scan.Messages {
		got, ok := defaultMessages[key]
		if !ok {
			return nil, fmt.Errorf("default catalog is out of date for %s; run 'aqi i18n gen'", key)
		}
		if item.Msg != "" && got != item.Msg {
			return nil, fmt.Errorf("default catalog is out of date for %s; run 'aqi i18n gen'", key)
		}
		if item.Msg == "" && strings.TrimSpace(got) == "" {
			warnings = append(warnings, fmt.Sprintf("%s: dynamic message needs manual translation", key))
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("i18n catalog directory not found; run 'aqi i18n gen'")
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		locale := strings.TrimSuffix(entry.Name(), ".yaml")
		messages, err := readCatalog(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if locale != language {
			missing := 0
			for key := range scan.Messages {
				if strings.TrimSpace(messages[key]) == "" {
					missing++
				}
			}
			if missing > 0 {
				warnings = append(warnings, fmt.Sprintf("%s: %d missing translations", locale, missing))
			}
		}

		orphan := 0
		for key := range messages {
			if _, ok := scan.Messages[key]; !ok {
				orphan++
			}
		}
		if orphan > 0 {
			warnings = append(warnings, fmt.Sprintf("%s: %d orphan keys", locale, orphan))
		}
	}
	sort.Strings(warnings)
	return warnings, nil
}

func scanSource(root string) (*sourceScan, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve source root: %w", err)
	}

	type parsedFile struct {
		path string
		fset *token.FileSet
		node *ast.File
	}
	filesByDir := make(map[string][]parsedFile)
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		dir := filepath.Dir(path)
		filesByDir[dir] = append(filesByDir[dir], parsedFile{path: path, fset: fset, node: node})
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &sourceScan{Messages: make(map[string]sourceMessage)}
	for _, files := range filesByDir {
		functions := make(map[string]*ast.FuncDecl)
		for _, file := range files {
			for _, decl := range file.node.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && fn.Body != nil {
					functions[fn.Name.Name] = fn
				}
			}
		}
		for _, file := range files {
			var scanErr error
			ast.Inspect(file.node, func(n ast.Node) bool {
				if scanErr != nil {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok || !isSelectorCall(call, "Add") || len(call.Args) < 2 {
					return true
				}
				action, ok := stringLiteral(call.Args[0])
				if !ok || action == "" {
					return true
				}

				var body *ast.BlockStmt
				switch handler := call.Args[1].(type) {
				case *ast.FuncLit:
					body = handler.Body
				case *ast.Ident:
					if fn := functions[handler.Name]; fn != nil {
						body = fn.Body
					}
				}
				if body == nil {
					return true
				}
				if err := collectHandlerMessages(result.Messages, action, body, file.path, file.fset); err != nil {
					scanErr = err
					return false
				}
				return true
			})
			if scanErr != nil {
				return nil, scanErr
			}
		}
	}
	return result, nil
}

func collectHandlerMessages(messages map[string]sourceMessage, action string, body *ast.BlockStmt, file string, fset *token.FileSet) error {
	var result error
	ast.Inspect(body, func(n ast.Node) bool {
		if result != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSelectorCall(call, "SendCode") || len(call.Args) < 2 {
			return true
		}
		code, ok := intLiteral(call.Args[0])
		if !ok {
			return true
		}
		msg, _ := stringLiteral(call.Args[1])
		position := fset.Position(call.Pos())
		current := sourceMessage{Action: action, Code: code, Msg: msg, File: file, Line: position.Line}
		key := messageKey(action, code)
		if previous, exists := messages[key]; exists {
			result = fmt.Errorf("duplicate i18n key %s: first at %s:%d, again at %s:%d", key, previous.File, previous.Line, current.File, current.Line)
			return false
		}
		messages[key] = current
		return true
	})
	return result
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".idea", ".vscode", "vendor", "node_modules", "data":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}

func isSelectorCall(call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func intLiteral(expr ast.Expr) (int, bool) {
	sign := 1
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.SUB {
		sign = -1
		expr = unary.X
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	value, err := strconv.ParseInt(lit.Value, 0, 64)
	if err != nil {
		return 0, false
	}
	return sign * int(value), true
}

func messageKey(action string, code int) string {
	return fmt.Sprintf("%s.%d", action, code)
}

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	parts := strings.Split(language, "-")
	if len(parts) == 0 || len(parts[0]) < 2 || len(parts[0]) > 8 || !lettersOnly(parts[0]) {
		return ""
	}
	for _, part := range parts[1:] {
		if len(part) == 0 || len(part) > 8 || !lettersOrDigits(part) {
			return ""
		}
	}
	return strings.Join(parts, "-")
}

func lettersOnly(value string) bool {
	for _, r := range value {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func lettersOrDigits(value string) bool {
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func readCatalog(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	messages := make(map[string]string)
	if len(data) == 0 {
		return messages, nil
	}
	if err := yaml.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return messages, nil
}

func marshalCatalog(messages map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(messages))
	for key := range messages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, key := range keys {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Value: messages[key]})
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeFileIfChanged(path string, data []byte) (bool, error) {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return false, err
	}
	return true, nil
}
