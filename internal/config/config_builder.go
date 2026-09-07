package config

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Builder provides ordered mutation helpers for the generated default YAML config.
// It uses yaml.Node internally so field order and comments are preserved.
type Builder struct {
	doc *yaml.Node
	err error
}

func NewBuilder(data []byte) (*Builder, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("default config root must be a mapping")
	}
	return &Builder{doc: &doc}, nil
}

func (b *Builder) Bytes() ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(b.doc); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *Builder) Err() error {
	return b.err
}

func (b *Builder) Has(path string) bool {
	_, _, ok := b.find(path)
	return ok
}

func (b *Builder) Get(path string) any {
	_, value, ok := b.find(path)
	if !ok {
		return nil
	}
	var out any
	if err := value.Decode(&out); err != nil {
		return nil
	}
	return out
}

func (b *Builder) Set(path string, value any) *Builder {
	if b.err != nil {
		return b
	}

	parent, key, err := b.ensureParent(path)
	if err != nil {
		return b.fail(err)
	}
	valueNode, err := encodeNode(value)
	if err != nil {
		return b.fail(err)
	}
	if index := mappingKeyIndex(parent, key); index >= 0 {
		parent.Content[index+1] = valueNode
		return b
	}
	appendMappingPair(parent, key, valueNode)
	return b
}

func (b *Builder) Delete(path string) *Builder {
	if b.err != nil {
		return b
	}

	parts := pathParts(path)
	if len(parts) == 0 {
		return b.fail(fmt.Errorf("invalid config path: %q", path))
	}

	parents := make([]*yaml.Node, 0, len(parts))
	current := b.doc.Content[0]
	for _, part := range parts[:len(parts)-1] {
		index := mappingKeyIndex(current, part)
		if index < 0 {
			return b
		}
		next := current.Content[index+1]
		if next.Kind != yaml.MappingNode {
			return b
		}
		parents = append(parents, current)
		current = next
	}

	index := mappingKeyIndex(current, parts[len(parts)-1])
	if index < 0 {
		return b
	}
	current.Content = append(current.Content[:index], current.Content[index+2:]...)

	// Prune empty parents so deleting mysql.logic does not leave mysql: {}.
	for i := len(parents) - 1; i >= 0 && len(current.Content) == 0; i-- {
		parent := parents[i]
		key := parts[i]
		parentIndex := mappingKeyIndex(parent, key)
		if parentIndex < 0 {
			break
		}
		parent.Content = append(parent.Content[:parentIndex], parent.Content[parentIndex+2:]...)
		current = parent
	}
	return b
}

// Comment sets a head comment on a config key. It overrides comments provided by struct tags.
func (b *Builder) Comment(path, comment string) *Builder {
	if b.err != nil {
		return b
	}

	parentPath, key := splitPath(path)
	if key == "" {
		return b.fail(fmt.Errorf("invalid config path: %q", path))
	}
	parent, err := b.mappingAt(parentPath)
	if err != nil {
		return b.fail(err)
	}
	index := mappingKeyIndex(parent, key)
	if index < 0 {
		return b.fail(fmt.Errorf("config path not found: %s", path))
	}
	parent.Content[index].HeadComment = strings.TrimSpace(comment)
	return b
}

// Before inserts path immediately before target. target and path must share the same parent.
func (b *Builder) Before(target, path string, value any) *Builder {
	return b.insertRelative(target, path, value, false)
}

// After inserts path immediately after target. target and path must share the same parent.
func (b *Builder) After(target, path string, value any) *Builder {
	return b.insertRelative(target, path, value, true)
}

// BeforeFields inserts all fields from a struct or mapping immediately before target.
// Struct fields keep declaration order, use yaml tags as keys, and support comment tags.
func (b *Builder) BeforeFields(target string, value any) *Builder {
	return b.insertFieldsRelative(target, value, false)
}

// AfterFields inserts all fields from a struct or mapping immediately after target.
// Struct fields keep declaration order, use yaml tags as keys, and support comment tags.
func (b *Builder) AfterFields(target string, value any) *Builder {
	return b.insertFieldsRelative(target, value, true)
}

func (b *Builder) insertFieldsRelative(target string, value any, after bool) *Builder {
	if b.err != nil {
		return b
	}

	parentPath, targetKey := splitPath(target)
	if targetKey == "" {
		return b.fail(fmt.Errorf("invalid target config path: %q", target))
	}

	parent, err := b.mappingAt(parentPath)
	if err != nil {
		return b.fail(err)
	}
	if mappingKeyIndex(parent, targetKey) < 0 {
		return b.fail(fmt.Errorf("target config path not found: %s", target))
	}

	fields, err := encodeNode(value)
	if err != nil {
		return b.fail(err)
	}
	if fields.Kind != yaml.MappingNode {
		return b.fail(fmt.Errorf("config fields must encode to a YAML mapping"))
	}
	applyStructComments(fields, value)

	for i := 0; i+1 < len(fields.Content); i += 2 {
		key := fields.Content[i].Value
		if key == targetKey {
			return b.fail(fmt.Errorf("config fields cannot replace insertion target: %s", target))
		}
		if existing := mappingKeyIndex(parent, key); existing >= 0 {
			parent.Content = append(parent.Content[:existing], parent.Content[existing+2:]...)
		}
	}

	targetIndex := mappingKeyIndex(parent, targetKey)
	insertAt := targetIndex
	if after {
		insertAt = targetIndex + 2
	}

	content := append([]*yaml.Node(nil), fields.Content...)
	parent.Content = append(parent.Content, make([]*yaml.Node, len(content))...)
	copy(parent.Content[insertAt+len(content):], parent.Content[insertAt:len(parent.Content)-len(content)])
	copy(parent.Content[insertAt:insertAt+len(content)], content)
	return b
}

func (b *Builder) insertRelative(target, path string, value any, after bool) *Builder {
	if b.err != nil {
		return b
	}

	targetParentPath, targetKey := splitPath(target)
	pathParentPath, pathKey := splitPath(path)
	if targetKey == "" || pathKey == "" || targetParentPath != pathParentPath {
		return b.fail(fmt.Errorf("target %q and path %q must share the same parent", target, path))
	}

	parent, err := b.mappingAt(targetParentPath)
	if err != nil {
		return b.fail(err)
	}
	targetIndex := mappingKeyIndex(parent, targetKey)
	if targetIndex < 0 {
		return b.fail(fmt.Errorf("target config path not found: %s", target))
	}

	valueNode, err := encodeNode(value)
	if err != nil {
		return b.fail(err)
	}

	if existing := mappingKeyIndex(parent, pathKey); existing >= 0 {
		parent.Content = append(parent.Content[:existing], parent.Content[existing+2:]...)
		targetIndex = mappingKeyIndex(parent, targetKey)
	}

	insertAt := targetIndex
	if after {
		insertAt = targetIndex + 2
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: pathKey}
	pair := []*yaml.Node{keyNode, valueNode}
	parent.Content = append(parent.Content, nil, nil)
	copy(parent.Content[insertAt+2:], parent.Content[insertAt:])
	copy(parent.Content[insertAt:insertAt+2], pair)
	return b
}

func (b *Builder) fail(err error) *Builder {
	if b.err == nil && err != nil {
		b.err = err
	}
	return b
}

func (b *Builder) find(path string) (*yaml.Node, *yaml.Node, bool) {
	parentPath, key := splitPath(path)
	if key == "" {
		return nil, nil, false
	}
	parent, err := b.mappingAt(parentPath)
	if err != nil {
		return nil, nil, false
	}
	index := mappingKeyIndex(parent, key)
	if index < 0 {
		return parent, nil, false
	}
	return parent, parent.Content[index+1], true
}

func (b *Builder) ensureParent(path string) (*yaml.Node, string, error) {
	parts := pathParts(path)
	if len(parts) == 0 {
		return nil, "", fmt.Errorf("invalid config path: %q", path)
	}
	current := b.doc.Content[0]
	for _, part := range parts[:len(parts)-1] {
		index := mappingKeyIndex(current, part)
		if index < 0 {
			next := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			appendMappingPair(current, part, next)
			current = next
			continue
		}
		next := current.Content[index+1]
		if next.Kind != yaml.MappingNode {
			return nil, "", fmt.Errorf("config path %q is not a mapping", part)
		}
		current = next
	}
	return current, parts[len(parts)-1], nil
}

func (b *Builder) mappingAt(path string) (*yaml.Node, error) {
	current := b.doc.Content[0]
	if path == "" {
		return current, nil
	}
	for _, part := range pathParts(path) {
		index := mappingKeyIndex(current, part)
		if index < 0 {
			return nil, fmt.Errorf("config path not found: %s", path)
		}
		current = current.Content[index+1]
		if current.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("config path is not a mapping: %s", path)
		}
	}
	return current, nil
}

func encodeNode(value any) (*yaml.Node, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	return &node, nil
}

func applyStructComments(node *yaml.Node, value any) {
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct || node == nil || node.Kind != yaml.MappingNode {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		key := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if key == "-" {
			continue
		}
		if key == "" {
			key = strings.ToLower(field.Name)
		}

		index := mappingKeyIndex(node, key)
		if index < 0 {
			continue
		}
		if comment := strings.TrimSpace(field.Tag.Get("comment")); comment != "" {
			node.Content[index].HeadComment = comment
		}
		applyStructComments(node.Content[index+1], v.Field(i).Interface())
	}
}

func mappingKeyIndex(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func appendMappingPair(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func splitPath(path string) (string, string) {
	parts := pathParts(path)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func pathParts(path string) []string {
	raw := strings.Split(strings.TrimSpace(path), ".")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
