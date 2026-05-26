package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"text/template"
)

//go:embed templates/*.tmpl
var templates embed.FS

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func main() {
	name := flag.String("name", "", "theme name (lowercase slug)")
	flag.Parse()

	if *name == "" {
		log.Fatal("new_theme: --name is required (e.g., make new-theme name=<theme name here>)")
	}

	if !nameRe.MatchString(*name) {
		log.Fatalf("new_theme: name %q must match ^[a-z][a-z0-9_]*$ (lowercase letters, digits, underscores; must start with a letter)", *name)
	}

	root := filepath.Join("themes", *name)

	if _, err := os.Stat(root); err == nil {
		log.Fatalf("new_theme: %s already exists, refusing to overwrite", root)
	}

	if err := createDirs(root); err != nil {
		log.Fatal(err)
	}

	if err := renderTemplates(root, map[string]string{"Name": *name}); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("themes/%s/ created. Next steps:\n", *name)
	fmt.Printf("  1. Edit themes/%s/theme.yml. Fill in Author.Name (DisplayName defaults to %q)\n", *name, *name+" Theme")
	fmt.Printf("  2. Edit themes/%s/config.yml to override Branding, Navbar, and other keys you care about.\n", *name)
	fmt.Printf("  3. Add files under themes/%s/ to override defaults.\n", *name)
	fmt.Printf("  4. For any pages added under themes/%s/pages/, add a matching ThemePages.<X> entry to themes/%s/access.yml\n", *name, *name)
	fmt.Printf("  5. Set 'Theme: %s' in conf/app.yml (under the App: block) when ready to activate\n", *name)
}

func createDirs(root string) error {
	dirs := []string{
		filepath.Join(root, "static", "css"),
		filepath.Join(root, "static", "vendor"),
		filepath.Join(root, "pages"),
		filepath.Join(root, "platform", "httpx"),
		filepath.Join(root, "features"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}

		gitkeep := filepath.Join(dir, ".gitkeep")

		if err := os.WriteFile(gitkeep, nil, 0o600); err != nil {
			return fmt.Errorf("touch %s: %w", gitkeep, err)
		}
	}

	return nil
}

func renderTemplates(root string, data map[string]string) error {
	files := map[string]string{
		"theme.yml":       "theme.yml.tmpl",
		"config.yml":      "config.yml.tmpl",
		"access.yml":      "access.yml.tmpl",
		"pages_embed.go":  "pages_embed.go.tmpl",
		"static_embed.go": "static_embed.go.tmpl",
		"access_embed.go": "access_embed.go.tmpl",
	}

	for outName, tmplName := range files {
		tmpl, err := template.ParseFS(templates, "templates/"+tmplName)
		if err != nil {
			return fmt.Errorf("parse %s: %w", tmplName, err)
		}

		var buf bytes.Buffer

		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("render %s: %w", tmplName, err)
		}

		path := filepath.Join(root, outName)

		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}
