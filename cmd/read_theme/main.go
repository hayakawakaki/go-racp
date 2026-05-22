package main

import (
	"cmp"
	"fmt"
	"os"
	"regexp"

	"github.com/goccy/go-yaml"
)

const defaultThemeName = "default"

var themeNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

func main() {
	data, err := os.ReadFile("config.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read_theme: %v\n", err)
		os.Exit(1)
	}

	var cfg struct {
		General struct {
			Theme string `yaml:"Theme"`
		} `yaml:"GeneralConfig"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "read_theme: %v\n", err)
		os.Exit(1)
	}

	name := cmp.Or(cfg.General.Theme, defaultThemeName)
	if !themeNameRe.MatchString(name) {
		fmt.Fprintf(os.Stderr, "read_theme: invalid theme name %q (must match %s)\n", name, themeNameRe)
		os.Exit(1)
	}

	fmt.Print(name)
}
