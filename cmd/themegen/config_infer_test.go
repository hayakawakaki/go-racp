package main

import (
	"strings"
	"testing"
)

func TestInferFromYAML_Scalars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantScalar string
		wantKind   Kind
	}{
		{name: "string", input: `key: "hello"`, wantKind: KindStruct},
		{name: "int", input: `key: 42`, wantKind: KindStruct},
		{name: "bool true", input: `key: true`, wantKind: KindStruct},
		{name: "bool false", input: `key: false`, wantKind: KindStruct},
		{name: "float", input: `key: 3.14`, wantKind: KindStruct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec, err := inferFromYAML([]byte(tt.input))
			if err != nil {
				t.Fatalf("inferFromYAML: %v", err)
			}
			if spec.Kind != tt.wantKind {
				t.Errorf("root Kind = %v, want %v", spec.Kind, tt.wantKind)
			}
			if len(spec.Fields) != 1 || spec.Fields[0].Name != "key" {
				t.Errorf("expected single field 'key', got %#v", spec.Fields)
			}
		})
	}
}

func TestInferFromYAML_ScalarTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "string quoted", yaml: `key: "hello"`, want: "string"},
		{name: "string bare", yaml: `key: hello`, want: "string"},
		{name: "int positive", yaml: `key: 42`, want: "int"},
		{name: "int negative", yaml: `key: -7`, want: "int"},
		{name: "bool true", yaml: `key: true`, want: "bool"},
		{name: "bool false", yaml: `key: false`, want: "bool"},
		{name: "float", yaml: `key: 3.14`, want: "float64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec, err := inferFromYAML([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("inferFromYAML: %v", err)
			}
			if got := spec.Fields[0].Type.Scalar; got != tt.want {
				t.Errorf("scalar type = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferFromYAML_NestedStruct(t *testing.T) {
	t.Parallel()

	input := `Branding:
  Logo: ""
  Discord: "x"
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	if len(spec.Fields) != 1 || spec.Fields[0].Name != "Branding" {
		t.Fatalf("want single top field 'Branding', got %#v", spec.Fields)
	}

	branding := spec.Fields[0].Type
	if branding.Kind != KindStruct {
		t.Fatalf("Branding Kind = %v, want struct", branding.Kind)
	}
	if len(branding.Fields) != 2 {
		t.Fatalf("Branding fields = %d, want 2", len(branding.Fields))
	}
	if branding.Fields[0].Name != "Logo" || branding.Fields[1].Name != "Discord" {
		t.Errorf("field order = %s, %s, want Logo, Discord", branding.Fields[0].Name, branding.Fields[1].Name)
	}
}

func TestInferFromYAML_SliceOfScalars(t *testing.T) {
	t.Parallel()

	input := `Colors: ["red", "blue", "green"]`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	colors := spec.Fields[0].Type
	if colors.Kind != KindSlice {
		t.Fatalf("Colors Kind = %v, want slice", colors.Kind)
	}
	if colors.Element == nil || colors.Element.Kind != KindScalar || colors.Element.Scalar != "string" {
		t.Errorf("Colors element = %#v, want scalar string", colors.Element)
	}
}

func TestInferFromYAML_SliceOfStructs(t *testing.T) {
	t.Parallel()

	input := `Items:
  - Label: "Home"
    Href: "/"
  - Label: "News"
    Href: "/news"
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	items := spec.Fields[0].Type
	if items.Kind != KindSlice {
		t.Fatalf("Items Kind = %v, want slice", items.Kind)
	}
	if items.Element == nil || items.Element.Kind != KindStruct {
		t.Fatalf("Items element = %#v, want struct", items.Element)
	}
	if len(items.Element.Fields) != 2 {
		t.Fatalf("element fields = %d, want 2", len(items.Element.Fields))
	}
}

func TestInferFromYAML_EmptyListWithNoSiblingErrorsAtFinalize(t *testing.T) {
	t.Parallel()

	spec, err := inferFromYAML([]byte(`Items: []`))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	types := finalize(spec)

	for _, ty := range types {
		for _, f := range ty.Fields {
			if f.Name == "Items" {
				if f.Type.Element != nil {
					t.Errorf("Items.Element = %#v, want nil (unresolved)", f.Type.Element)
				}
			}
		}
	}
}

func TestInferFromYAML_HeterogeneousListErrors(t *testing.T) {
	t.Parallel()

	_, err := inferFromYAML([]byte(`Mixed: [1, "two", true]`))
	if err == nil {
		t.Fatal("expected error on heterogeneous list, got nil")
	}
	if !strings.Contains(err.Error(), "heterogeneous") {
		t.Errorf("error = %q, want substring %q", err.Error(), "heterogeneous")
	}
}

func TestInferFromYAML_EmptyYAMLProducesEmptyConfig(t *testing.T) {
	t.Parallel()

	spec, err := inferFromYAML([]byte(``))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}
	if spec.Kind != KindStruct {
		t.Errorf("Kind = %v, want struct", spec.Kind)
	}
	if len(spec.Fields) != 0 {
		t.Errorf("Fields = %d, want 0", len(spec.Fields))
	}
}

func TestFinalize_AssignsTopLevelStructNames(t *testing.T) {
	t.Parallel()

	input := `Branding:
  Logo: ""
Navbar:
  Items:
    - Label: "Home"
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	types := finalize(spec)

	names := map[string]bool{}
	for _, ty := range types {
		names[ty.Name] = true
	}
	for _, want := range []string{"Config", "Branding", "Navbar", "NavbarItem"} {
		if !names[want] {
			t.Errorf("missing type %q in %v", want, namesOf(types))
		}
	}
}

func TestFinalize_DetectsRecursion(t *testing.T) {
	t.Parallel()

	input := `Navbar:
  Items:
    - Label: "Home"
      Href: "/"
      Icon: "home"
      Children:
        - Label: "Sub"
          Href: "/sub"
          Icon: "x"
          Children: []
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	types := finalize(spec)

	navbarItemCount := 0
	for _, ty := range types {
		if ty.Name == "NavbarItem" {
			navbarItemCount++
		}
	}
	if navbarItemCount != 1 {
		t.Errorf("expected exactly 1 NavbarItem after recursion collapse, got %d", navbarItemCount)
	}
}

func TestFinalize_RecursionViaPopulatedChildren(t *testing.T) {
	t.Parallel()

	input := `Navbar:
  Items:
    - Label: "DB"
      Href: ""
      Icon: "db"
      Children:
        - Label: "Items"
          Href: "/items"
          Icon: "x"
          Children:
            - Label: "Equip"
              Href: "/items/equip"
              Icon: "x"
              Children: []
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	types := finalize(spec)

	navbarItemCount := 0
	for _, ty := range types {
		if ty.Name == "NavbarItem" {
			navbarItemCount++
		}
	}
	if navbarItemCount != 1 {
		t.Errorf("expected exactly 1 NavbarItem after recursion collapse, got %d", navbarItemCount)
	}
}

func TestFinalize_RecursionWithNonEmptyTerminal(t *testing.T) {
	t.Parallel()

	input := `Navbar:
  Items:
    - Label: "DB"
      Href: ""
      Icon: "db"
      Children:
        - Label: "Items"
          Href: "/items"
          Icon: "x"
          Children:
            - Label: "Equip"
              Href: "/equip"
              Icon: "x"
              Children:
                - Label: "Sword"
                  Href: "/sword"
                  Icon: "x"
                  Children:
                    - Label: "OneHand"
                      Href: "/onehand"
                      Icon: "x"
                      Children:
                        - Label: "Dagger"
                          Href: "/dagger"
                          Icon: "x"
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	types := finalize(spec)

	navbarItemCount := 0
	for _, ty := range types {
		if ty.Name == "NavbarItem" {
			navbarItemCount++
		}
	}
	if navbarItemCount != 1 {
		t.Errorf("expected exactly 1 NavbarItem type after dedup, got %d (types: %v)", navbarItemCount, namesOf(types))
	}

	for _, ty := range types {
		if ty.Name != "NavbarItem" {
			continue
		}
		var childrenField *FieldSpec
		for i := range ty.Fields {
			if ty.Fields[i].Name == "Children" {
				childrenField = &ty.Fields[i]
				break
			}
		}
		if childrenField == nil {
			t.Fatal("NavbarItem missing Children field")
		}
		if childrenField.Type.Kind != KindSlice {
			t.Fatalf("NavbarItem.Children Kind = %v, want slice", childrenField.Type.Kind)
		}
		if childrenField.Type.Element.Name != "NavbarItem" {
			t.Errorf("NavbarItem.Children element name = %q, want %q (recursion not collapsed)", childrenField.Type.Element.Name, "NavbarItem")
		}
	}
}

func namesOf(types []TypeSpec) []string {
	out := make([]string, 0, len(types))
	for _, ty := range types {
		out = append(out, ty.Name)
	}
	return out
}

func TestInferFromYAML_EmptyListResolvedByPopulatedSibling(t *testing.T) {
	t.Parallel()

	input := `Navbar:
  Items:
    - Label: "Home"
      Href: "/"
      Icon: "home"
      Children: []
    - Label: "DB"
      Href: ""
      Icon: "db"
      Children:
        - Label: "Items"
          Href: "/items"
          Icon: ""
          Children: []
`
	spec, err := inferFromYAML([]byte(input))
	if err != nil {
		t.Fatalf("inferFromYAML: %v", err)
	}

	types := finalize(spec)

	for _, ty := range types {
		if ty.Name != "NavbarItem" {
			continue
		}
		for _, f := range ty.Fields {
			if f.Name != "Children" {
				continue
			}
			if f.Type.Kind != KindSlice {
				t.Fatalf("Children Kind = %v, want slice", f.Type.Kind)
			}
			if f.Type.Element == nil {
				t.Fatal("Children element nil after finalize; expected resolution from populated sibling")
			}
			if f.Type.Element.Name != "NavbarItem" {
				t.Errorf("Children element = %q, want NavbarItem", f.Type.Element.Name)
			}
		}
	}
}
