package lockfile

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type PubspecLockDescription struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Path string `yaml:"path"`
	Ref  string `yaml:"resolved-ref"`
}

var _ yaml.Unmarshaler = &PubspecLockDescription{}

func (pld *PubspecLockDescription) UnmarshalYAML(value *yaml.Node) error {
	var m struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
		Path string `yaml:"path"`
		Ref  string `yaml:"resolved-ref"`
	}

	err := value.Decode(&m)

	if err == nil {
		pld.Name = m.Name
		pld.Path = m.Path
		pld.URL = m.URL
		pld.Ref = m.Ref

		return nil
	}

	var str *string

	err = value.Decode(&str)

	if err != nil {
		return err
	}

	pld.Path = *str

	return nil
}

type PubspecLockPackage struct {
	Source      string                 `yaml:"source"`
	Description PubspecLockDescription `yaml:"description"`
	Version     string                 `yaml:"version"`
}

type PubspecLockfile struct {
	Packages map[string]PubspecLockPackage `yaml:"packages,omitempty"`
	Sdks     map[string]string             `yaml:"sdks"`
}

const PubEcosystem Ecosystem = "Pub"

type PubspecLockExtractor struct{}

func (e PubspecLockExtractor) ShouldExtract(path string) bool {
	return filepath.Base(path) == "pubspec.lock"
}

func (e PubspecLockExtractor) Extract(f DepFile) ([]PackageDetails, error) {
	var parsedLockfile *PubspecLockfile

	err := yaml.NewDecoder(f).Decode(&parsedLockfile)

	if err != nil && !errors.Is(err, io.EOF) {
		return []PackageDetails{}, fmt.Errorf("could not parse %s: %w", f.Path(), err)
	}
	if parsedLockfile == nil {
		return []PackageDetails{}, nil
	}

	packages := make([]PackageDetails, 0, len(parsedLockfile.Packages))

	for name, pkg := range parsedLockfile.Packages {
		packages = append(packages, PackageDetails{
			Name:      name,
			Version:   pkg.Version,
			Commit:    pkg.Description.Ref,
			Ecosystem: PubEcosystem,
		})
	}

	return packages, nil
}

var _ Extractor = PubspecLockExtractor{}

func ParsePubspecLock(pathToLockfile string) ([]PackageDetails, error) {
	return parseFile(pathToLockfile, PubspecLockExtractor{})
}
