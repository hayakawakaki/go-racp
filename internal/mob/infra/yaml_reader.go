package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/goccy/go-yaml"
)

type DropInput struct {
	Item              string `yaml:"Item"`
	RandomOptionGroup string `yaml:"RandomOptionGroup"`
	Rate              int    `yaml:"Rate"`
	Index             int    `yaml:"Index"`
	StealProtected    bool   `yaml:"StealProtected"`
}

//nolint:govet // large struct sourced 1:1 from YAML
type YAMLInput struct {
	Modes           map[string]bool `yaml:"Modes"`
	Drops           []DropInput     `yaml:"Drops"`
	MvpDrops        []DropInput     `yaml:"MvpDrops"`
	AegisName       string          `yaml:"AegisName"`
	Name            string          `yaml:"Name"`
	JapaneseName    string          `yaml:"JapaneseName"`
	Title           string          `yaml:"Title"`
	Race            string          `yaml:"Race"`
	Element         string          `yaml:"Element"`
	Size            string          `yaml:"Size"`
	ID              int             `yaml:"Id"`
	Level           int             `yaml:"Level"`
	HP              int             `yaml:"Hp"`
	BaseExp         int             `yaml:"BaseExp"`
	JobExp          int             `yaml:"JobExp"`
	MvpExp          int             `yaml:"MvpExp"`
	Attack          int             `yaml:"Attack"`
	Attack2         int             `yaml:"Attack2"`
	Defense         int             `yaml:"Defense"`
	MagicDefense    int             `yaml:"MagicDefense"`
	Resistance      int             `yaml:"Resistance"`
	MagicResistance int             `yaml:"MagicResistance"`
	Str             int             `yaml:"Str"`
	Agi             int             `yaml:"Agi"`
	Vit             int             `yaml:"Vit"`
	Int             int             `yaml:"Int"`
	Dex             int             `yaml:"Dex"`
	Luk             int             `yaml:"Luk"`
	AttackRange     int             `yaml:"AttackRange"`
	SkillRange      int             `yaml:"SkillRange"`
	ChaseRange      int             `yaml:"ChaseRange"`
	ElementLevel    int             `yaml:"ElementLevel"`
	WalkSpeed       int             `yaml:"WalkSpeed"`
	AttackDelay     int             `yaml:"AttackDelay"`
	AttackMotion    int             `yaml:"AttackMotion"`
	DamageMotion    int             `yaml:"DamageMotion"`
	DamageTaken     int             `yaml:"DamageTaken"`
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
