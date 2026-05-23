package theme

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/goccy/go-yaml"
)

type Manifest struct {
	Name        string                `yaml:"Name"`
	DisplayName string                `yaml:"DisplayName"`
	Version     string                `yaml:"Version"`
	Author      ManifestAuthor        `yaml:"Author"`
	Compatible  ManifestCompatibility `yaml:"Compatible"`
}

type ManifestAuthor struct {
	Name string `yaml:"Name"`
	URL  string `yaml:"Url"`
}

type ManifestCompatibility struct {
	Min string `yaml:"Min"`
}

var (
	nameRe    = regexp.MustCompile(`^[a-z0-9_]+$`)
	versionRe = regexp.MustCompile(`^(\d+)\.(\d+)$`)
)

func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest

	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}

	if m.Name == "" {
		return Manifest{}, fmt.Errorf("manifest: Name is required")
	}

	if !nameRe.MatchString(m.Name) {
		return Manifest{}, fmt.Errorf("manifest: Name %q must match %s", m.Name, nameRe)
	}

	if m.DisplayName == "" {
		return Manifest{}, fmt.Errorf("manifest: DisplayName is required")
	}

	if _, _, err := ParseVersion(m.Version); err != nil {
		return Manifest{}, fmt.Errorf("manifest: Version: %w", err)
	}

	if m.Author.Name == "" {
		return Manifest{}, fmt.Errorf("manifest: Author.Name is required")
	}

	if _, _, err := ParseVersion(m.Compatible.Min); err != nil {
		return Manifest{}, fmt.Errorf("manifest: Compatible.Min: %w", err)
	}

	return m, nil
}

func ParseVersion(s string) (major, minor int, err error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("version %q must be in <major>.<minor> form", s)
	}

	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])

	return major, minor, nil
}

func VersionAtLeast(actual, minimum string) (bool, error) {
	aMaj, aMin, err := ParseVersion(actual)
	if err != nil {
		return false, fmt.Errorf("actual: %w", err)
	}

	bMaj, bMin, err := ParseVersion(minimum)
	if err != nil {
		return false, fmt.Errorf("minimum: %w", err)
	}

	if aMaj != bMaj {
		return aMaj > bMaj, nil
	}

	return aMin >= bMin, nil
}
