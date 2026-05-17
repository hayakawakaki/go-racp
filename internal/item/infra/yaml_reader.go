package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/goccy/go-yaml"
)

type YAMLInput struct {
	Jobs          map[string]bool `yaml:"Jobs"`
	Classes       map[string]bool `yaml:"Classes"`
	Locations     map[string]bool `yaml:"Locations"`
	Trade         map[string]any  `yaml:"Trade"`
	AegisName     string          `yaml:"AegisName"`
	Name          string          `yaml:"Name"`
	Type          string          `yaml:"Type"`
	SubType       string          `yaml:"SubType"`
	Gender        string          `yaml:"Gender"`
	Buy           int             `yaml:"Buy"`
	Sell          int             `yaml:"Sell"`
	Weight        int             `yaml:"Weight"`
	ID            int             `yaml:"Id"`
	View          int             `yaml:"View"`
	EquipLevelMin int             `yaml:"EquipLevelMin"`
	EquipLevelMax int             `yaml:"EquipLevelMax"`
	Slots         int             `yaml:"Slots"`
	Attack        int             `yaml:"Attack"`
	MagicAttack   int             `yaml:"MagicAttack"`
	Defense       int             `yaml:"Defense"`
	WeaponLevel   int             `yaml:"WeaponLevel"`
	Range         int             `yaml:"Range"`
	ArmorLevel    int             `yaml:"ArmorLevel"`
	Refineable    bool            `yaml:"Refineable"`
	Gradable      bool            `yaml:"Gradable"`
}

type yamlFile struct {
	Body []YAMLInput `yaml:"Body"`
}

func ReadYAML(path string) ([]YAMLInput, error) {
	//nolint:gosec // G304: paths come from operator-controlled config.yml.
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fs.ErrNotExist
		}

		return nil, fmt.Errorf("infra.ReadYAML: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var parsed yamlFile
	if err := yaml.UnmarshalWithOptions(data, &parsed, yaml.AllowDuplicateMapKey()); err != nil {
		return nil, fmt.Errorf("infra.ReadYAML: %w", err)
	}

	return parsed.Body, nil
}
