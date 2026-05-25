package main

import (
	"fmt"
	"strings"
)

func emitConfigGen(themeName string, types []TypeSpec) string {
	var b strings.Builder

	b.WriteString(generatedHeader)
	b.WriteString("package themecfg\n\n")

	ordered := orderTypes(types)

	maxName := 0
	for _, ty := range ordered {
		for _, f := range ty.Fields {
			if len(f.Name) > maxName {
				maxName = len(f.Name)
			}
		}
	}

	for i, ty := range ordered {
		emitStruct(&b, ty, maxName)
		if i < len(ordered)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\nvar Cfg Config\n")

	_ = themeName

	return b.String()
}

func orderTypes(types []TypeSpec) []TypeSpec {
	var cfg *TypeSpec
	var rest []TypeSpec
	for i := range types {
		if types[i].Name == configTypeName {
			cfg = &types[i]
			continue
		}
		rest = append(rest, types[i])
	}
	out := make([]TypeSpec, 0, len(types))
	if cfg != nil {
		out = append(out, *cfg)
	}
	out = append(out, rest...)
	return out
}

func emitStruct(b *strings.Builder, ty TypeSpec, maxName int) {
	fmt.Fprintf(b, "type %s struct {\n", ty.Name)

	maxType := 0
	for _, f := range ty.Fields {
		if t := goType(f.Type); len(t) > maxType {
			maxType = len(t)
		}
	}

	for _, f := range ty.Fields {
		pad := strings.Repeat(" ", maxName-len(f.Name))
		typeStr := goType(f.Type)
		typePad := strings.Repeat(" ", maxType-len(typeStr))
		fmt.Fprintf(b, "\t%s%s %s%s `yaml:%q`\n", f.Name, pad, typeStr, typePad, f.Name)
	}
	b.WriteString("}\n")
}

func goType(spec *TypeSpec) string {
	switch spec.Kind {
	case KindScalar:
		return spec.Scalar
	case KindSlice:
		if spec.Element == nil {
			return "[]any"
		}
		return "[]" + goType(spec.Element)
	case KindStruct:
		return spec.Name
	}
	return "any"
}
