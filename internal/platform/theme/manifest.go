package theme

type Manifest struct {
	Name        string
	DisplayName string
	Version     string
	Author      ManifestAuthor
	Compatible  ManifestCompatibility
	Preview     string
}

type ManifestAuthor struct {
	Name string
	URL  string `yaml:"Url"`
}

type ManifestCompatibility struct {
	Min string
}
