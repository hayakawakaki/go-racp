package main

import (
	"fmt"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

type Kind int

const childrenKey = "Children"

const (
	KindScalar Kind = iota
	KindStruct
	KindSlice
	KindNull
)

type FieldSpec struct {
	Type *TypeSpec
	Name string
}

type TypeSpec struct {
	Element *TypeSpec
	Name    string
	Scalar  string
	Fields  []FieldSpec
	Kind    Kind
}

func inferFromYAML(data []byte) (*TypeSpec, error) {
	root := &TypeSpec{Kind: KindStruct}
	if len(data) == 0 {
		return root, nil
	}

	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if len(file.Docs) == 0 {
		return root, nil
	}

	body := file.Docs[0].Body
	if body == nil {
		return root, nil
	}

	return inferNode(body)
}

func inferNode(node ast.Node) (*TypeSpec, error) {
	switch n := node.(type) {
	case *ast.MappingNode:
		return inferMapping(n)
	case *ast.SequenceNode:
		return inferSequence(n)
	case *ast.StringNode:
		return &TypeSpec{Kind: KindScalar, Scalar: "string"}, nil
	case *ast.IntegerNode:
		return &TypeSpec{Kind: KindScalar, Scalar: "int"}, nil
	case *ast.FloatNode:
		return &TypeSpec{Kind: KindScalar, Scalar: "float64"}, nil
	case *ast.BoolNode:
		return &TypeSpec{Kind: KindScalar, Scalar: "bool"}, nil
	case *ast.NullNode:
		return &TypeSpec{Kind: KindNull}, nil
	case *ast.MappingValueNode:
		return inferNode(n.Value)
	default:
		return nil, fmt.Errorf("unsupported yaml node type %T at %s", node, node.GetPath())
	}
}

func inferMapping(node *ast.MappingNode) (*TypeSpec, error) {
	out := &TypeSpec{Kind: KindStruct}
	for _, value := range node.Values {
		key, ok := value.Key.(*ast.StringNode)
		if !ok {
			return nil, fmt.Errorf("non-string mapping key at %s", value.GetPath())
		}

		fieldType, err := inferNode(value.Value)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", key.Value, err)
		}

		out.Fields = append(out.Fields, FieldSpec{Name: key.Value, Type: fieldType})
	}

	return out, nil
}

func inferSequence(node *ast.SequenceNode) (*TypeSpec, error) {
	if len(node.Values) == 0 {
		return &TypeSpec{Kind: KindSlice, Element: nil}, nil
	}

	elements := make([]*TypeSpec, 0, len(node.Values))
	for i, v := range node.Values {
		spec, err := inferNode(v)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		elements = append(elements, spec)
	}

	merged, err := mergeShapes(elements)
	if err != nil {
		return nil, fmt.Errorf("at %s: %w", node.GetPath(), err)
	}

	return &TypeSpec{Kind: KindSlice, Element: merged}, nil
}

func mergeShapes(specs []*TypeSpec) (*TypeSpec, error) {
	nonNull := filterNonNull(specs)
	if len(nonNull) == 0 {
		return &TypeSpec{Kind: KindNull}, nil
	}

	base := nonNull[0]
	for _, s := range nonNull[1:] {
		if s.Kind != base.Kind {
			return nil, fmt.Errorf("heterogeneous list: mixed kinds")
		}
	}

	return mergeByKind(base, nonNull)
}

func filterNonNull(specs []*TypeSpec) []*TypeSpec {
	out := make([]*TypeSpec, 0, len(specs))
	for _, s := range specs {
		if s.Kind != KindNull {
			out = append(out, s)
		}
	}

	return out
}

func mergeByKind(base *TypeSpec, specs []*TypeSpec) (*TypeSpec, error) {
	switch base.Kind {
	case KindScalar:
		for _, s := range specs[1:] {
			if s.Scalar != base.Scalar {
				return nil, fmt.Errorf("heterogeneous list: mixed scalar types (%s, %s)", base.Scalar, s.Scalar)
			}
		}
		return base, nil
	case KindStruct:
		return mergeStructFields(specs)
	case KindSlice:
		return mergeSliceElements(specs)
	}

	return base, nil
}

func mergeStructFields(specs []*TypeSpec) (*TypeSpec, error) {
	order := []string{}
	groups := map[string][]*TypeSpec{}

	for _, spec := range specs {
		for _, f := range spec.Fields {
			if _, seen := groups[f.Name]; !seen {
				order = append(order, f.Name)
			}
			groups[f.Name] = append(groups[f.Name], f.Type)
		}
	}

	out := &TypeSpec{Kind: KindStruct}
	for _, name := range order {
		merged, err := mergeShapes(groups[name])
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		out.Fields = append(out.Fields, FieldSpec{Name: name, Type: merged})
	}

	return out, nil
}

func mergeSliceElements(specs []*TypeSpec) (*TypeSpec, error) {
	elements := []*TypeSpec{}
	for _, s := range specs {
		if s.Element != nil {
			elements = append(elements, s.Element)
		}
	}
	if len(elements) == 0 {
		return &TypeSpec{Kind: KindSlice, Element: nil}, nil
	}

	merged, err := mergeShapes(elements)
	if err != nil {
		return nil, err
	}

	return &TypeSpec{Kind: KindSlice, Element: merged}, nil
}

func sameShape(a, b *TypeSpec) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindScalar:
		return a.Scalar == b.Scalar
	case KindSlice:
		if a.Element == nil || b.Element == nil {
			return true
		}
		return sameShape(a.Element, b.Element)
	case KindStruct:
		return sameStruct(a, b)
	}

	return false
}

func sameStruct(a, b *TypeSpec) bool {
	if len(a.Fields) != len(b.Fields) {
		return false
	}
	bByName := map[string]*TypeSpec{}
	for _, f := range b.Fields {
		bByName[f.Name] = f.Type
	}
	for _, f := range a.Fields {
		bt, ok := bByName[f.Name]
		if !ok {
			return false
		}
		if !sameShape(f.Type, bt) {
			return false
		}
	}

	return true
}

const configTypeName = "Config"

func finalize(root *TypeSpec) []TypeSpec {
	root.Name = configTypeName
	assignNames(root, "")
	resolveEmptySlices(root)
	deduplicateStructs(root)
	collapseSubsetStructs(root)
	return collectStructs(root)
}

func collapseSubsetStructs(root *TypeSpec) {
	all := []*TypeSpec{}
	visit(root, func(spec *TypeSpec) {
		if spec.Kind == KindStruct {
			all = append(all, spec)
		}
	})

	pass := func() bool {
		changed := false
		for _, smaller := range all {
			for _, larger := range all {
				if smaller == larger || smaller.Name == larger.Name {
					continue
				}
				if len(smaller.Fields) >= len(larger.Fields) {
					continue
				}
				if !isSubsetStruct(smaller, larger) {
					continue
				}
				smaller.Name = larger.Name
				smaller.Fields = larger.Fields
				changed = true
				break
			}
		}

		return changed
	}
	for pass() {
	}
}

func isSubsetStruct(small, large *TypeSpec) bool {
	largeByName := map[string]*TypeSpec{}
	for _, f := range large.Fields {
		largeByName[f.Name] = f.Type
	}
	for _, f := range small.Fields {
		lt, ok := largeByName[f.Name]
		if !ok {
			return false
		}
		if !sameShape(f.Type, lt) {
			return false
		}
	}

	return true
}

func resolveEmptySlices(root *TypeSpec) {
	candidates := collectSliceCandidates(root)
	applySliceCandidates(root, candidates)
}

func collectSliceCandidates(root *TypeSpec) map[string]*TypeSpec {
	candidates := map[string]*TypeSpec{}
	visit(root, func(spec *TypeSpec) {
		if spec.Kind != KindStruct {
			return
		}
		for i := range spec.Fields {
			field := &spec.Fields[i]
			if field.Type.Kind == KindSlice && field.Type.Element != nil {
				if _, ok := candidates[field.Name]; !ok {
					candidates[field.Name] = field.Type.Element
				}
			}
		}
	})

	return candidates
}

func applySliceCandidates(root *TypeSpec, candidates map[string]*TypeSpec) {
	visit(root, func(spec *TypeSpec) {
		if spec.Kind != KindStruct {
			return
		}
		for i := range spec.Fields {
			field := &spec.Fields[i]
			resolved, ok := candidates[field.Name]
			if !ok {
				continue
			}
			switch field.Type.Kind {
			case KindSlice:
				if field.Type.Element == nil {
					field.Type.Element = resolved
				}
			case KindNull:
				field.Type = &TypeSpec{Kind: KindSlice, Element: resolved}
			}
		}
	})
}

//nolint:cyclop // tree walk branches per child kind
func assignNames(spec *TypeSpec, parentName string) {
	switch spec.Kind {
	case KindStruct:
		for i := range spec.Fields {
			field := &spec.Fields[i]
			child := field.Type
			switch child.Kind {
			case KindStruct:
				if child.Name == "" {
					child.Name = field.Name
				}
				assignNames(child, child.Name)
			case KindSlice:
				if child.Element != nil && child.Element.Kind == KindStruct && child.Element.Name == "" {
					base := parentName
					if base == "" || base == configTypeName {
						base = field.Name
					}
					child.Element.Name = singularize(base, field.Name)
					assignNames(child.Element, child.Element.Name)
				}
			}
		}
	case KindSlice:
		if spec.Element != nil {
			assignNames(spec.Element, parentName)
		}
	}
}

func singularize(parent, field string) string {
	if field == childrenKey && parent != "" && parent != configTypeName {
		return parent
	}
	candidate := field
	if len(candidate) > 1 && candidate[len(candidate)-1] == 's' {
		candidate = candidate[:len(candidate)-1]
	}
	if candidate == parent || candidate == "" {
		return parent + "Item"
	}
	if parent == "" || parent == configTypeName {
		return candidate
	}
	return parent + candidate
}

func deduplicateStructs(root *TypeSpec) {
	canonical := map[string]*TypeSpec{}
	pass := func() bool {
		changed := false
		visit(root, func(spec *TypeSpec) {
			if spec.Kind != KindStruct {
				return
			}
			key := shapeKey(spec)
			existing, ok := canonical[key]
			if !ok {
				canonical[key] = spec
				return
			}
			if existing == spec {
				return
			}
			if spec.Name != existing.Name {
				spec.Name = existing.Name
				changed = true
			}
		})
		return changed
	}
	for pass() {
	}
}

func shapeKey(spec *TypeSpec) string {
	if spec.Kind != KindStruct {
		return ""
	}
	var b []byte
	for _, f := range spec.Fields {
		b = append(b, f.Name...)
		b = append(b, ':')
		b = append(b, typeTag(f.Type)...)
		b = append(b, ';')
	}
	return string(b)
}

func typeTag(spec *TypeSpec) string {
	if spec == nil {
		return "nil"
	}
	switch spec.Kind {
	case KindScalar:
		return "s:" + spec.Scalar
	case KindSlice:
		if spec.Element == nil {
			return "[]nil"
		}
		return "[]" + typeTag(spec.Element)
	case KindStruct:
		return "@" + spec.Name
	}
	return "?"
}

func visit(spec *TypeSpec, fn func(*TypeSpec)) {
	visitWith(spec, fn, map[*TypeSpec]bool{})
}

func visitWith(spec *TypeSpec, fn func(*TypeSpec), seen map[*TypeSpec]bool) {
	if spec == nil || seen[spec] {
		return
	}
	seen[spec] = true
	fn(spec)
	switch spec.Kind {
	case KindStruct:
		for i := range spec.Fields {
			visitWith(spec.Fields[i].Type, fn, seen)
		}
	case KindSlice:
		visitWith(spec.Element, fn, seen)
	}
}

func collectStructs(root *TypeSpec) []TypeSpec {
	seen := map[string]bool{}
	var out []TypeSpec
	visit(root, func(spec *TypeSpec) {
		if spec.Kind != KindStruct {
			return
		}
		if seen[spec.Name] {
			return
		}
		seen[spec.Name] = true
		out = append(out, *spec)
	})
	return out
}
