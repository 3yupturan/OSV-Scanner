package lockfile

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/osv-scanner/pkg/models"

	"github.com/google/osv-scanner/internal/cachedregexp"
	"gopkg.in/yaml.v3"
)

type PnpmLockPackageResolution struct {
	Tarball string `yaml:"tarball"`
	Commit  string `yaml:"commit"`
	Repo    string `yaml:"repo"`
	Type    string `yaml:"type"`
}

type PnpmLockPackage struct {
	Resolution PnpmLockPackageResolution `yaml:"resolution"`
	Name       string                    `yaml:"name"`
	Version    string                    `yaml:"version"`
	Dev        bool                      `yaml:"dev"`
	models.FilePosition
}

type PnpmLockPackages map[string]PnpmLockPackage

type PnpmLockfile struct {
	Version  float64          `yaml:"lockfileVersion"`
	Packages PnpmLockPackages `yaml:"packages,omitempty"`
}

type PnpmLockfileV6 struct {
	Version  string           `yaml:"lockfileVersion"`
	Packages PnpmLockPackages `yaml:"packages,omitempty"`
}

func (l *PnpmLockfile) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var lockfileV6 PnpmLockfileV6

	if err := unmarshal(&lockfileV6); err != nil {
		return err
	}

	parsedVersion, err := strconv.ParseFloat(lockfileV6.Version, 64)

	if err != nil {
		return err
	}

	l.Version = parsedVersion
	l.Packages = lockfileV6.Packages

	return nil
}

func (pnpmLockPackages *PnpmLockPackages) UnmarshalYAML(value *yaml.Node) error {
	if *pnpmLockPackages == nil {
		*pnpmLockPackages = make(map[string]PnpmLockPackage)
	}

	for i := 0; i < len(value.Content); i += 2 {
		var pnpmLockPackage PnpmLockPackage
		keyNode := value.Content[i]
		valueNode := value.Content[i+1]

		// If is empty, start/end line are the same
		pnpmLockPackage.SetLineStart(keyNode.Line)
		pnpmLockPackage.SetLineEnd(keyNode.Line)

		// Is not empty
		if valueNode.Kind == yaml.MappingNode {
			pnpmLockPackage.SetLineEnd(valueNode.Content[len(valueNode.Content)-1].Line)
			if err := valueNode.Decode(&pnpmLockPackage); err != nil {
				return err
			}
		}

		(*pnpmLockPackages)[keyNode.Value] = pnpmLockPackage
	}

	return nil
}

const PnpmEcosystem = NpmEcosystem

func startsWithNumber(str string) bool {
	matcher := cachedregexp.MustCompile(`^\d`)

	return matcher.MatchString(str)
}

// extractPnpmPackageNameAndVersion parses a dependency path, attempting to
// extract the name and version of the package it represents
func extractPnpmPackageNameAndVersion(dependencyPath string) (string, string) {
	// file dependencies must always have a name property to be installed,
	// and their dependency path never has the version encoded, so we can
	// skip trying to extract either from their dependency path
	if strings.HasPrefix(dependencyPath, "file:") {
		return "", ""
	}

	parts := strings.Split(dependencyPath, "/")
	var name string

	parts = parts[1:]

	if strings.HasPrefix(parts[0], "@") {
		name = strings.Join(parts[:2], "/")
		parts = parts[2:]
	} else {
		name = parts[0]
		parts = parts[1:]
	}

	version := ""

	if len(parts) != 0 {
		version = parts[0]
	}

	if version == "" {
		name, version = parseNameAtVersion(name)
	}

	if version == "" || !startsWithNumber(version) {
		return "", ""
	}

	underscoreIndex := strings.Index(version, "_")

	if underscoreIndex != -1 {
		version = strings.Split(version, "_")[0]
	}

	return name, version
}

func parseNameAtVersion(value string) (name string, version string) {
	// look for pattern "name@version", where name is allowed to contain zero or more "@"
	matches := cachedregexp.MustCompile(`^(.+)@([\d.]+)$`).FindStringSubmatch(value)

	if len(matches) != 3 {
		return name, ""
	}

	return matches[1], matches[2]
}

func parsePnpmLock(lockfile PnpmLockfile) []PackageDetails {
	packages := make([]PackageDetails, 0, len(lockfile.Packages))

	for s, pkg := range lockfile.Packages {
		name, version := extractPnpmPackageNameAndVersion(s)

		// "name" is only present if it's not in the dependency path and takes
		// priority over whatever name we think we've extracted (if any)
		if pkg.Name != "" {
			name = pkg.Name
		}

		// "version" is only present if it's not in the dependency path and takes
		// priority over whatever version we think we've extracted (if any)
		if pkg.Version != "" {
			version = pkg.Version
		}

		if name == "" || version == "" {
			continue
		}

		commit := pkg.Resolution.Commit

		if strings.HasPrefix(pkg.Resolution.Tarball, "https://codeload.github.com") {
			re := cachedregexp.MustCompile(`https://codeload\.github\.com(?:/[\w-.]+){2}/tar\.gz/(\w+)$`)
			matched := re.FindStringSubmatch(pkg.Resolution.Tarball)

			if matched != nil {
				commit = matched[1]
			}
		}

		var depGroups []string
		if pkg.Dev {
			depGroups = append(depGroups, "dev")
		}

		packages = append(packages, PackageDetails{
			Name:      name,
			Version:   version,
			Ecosystem: PnpmEcosystem,
			CompareAs: PnpmEcosystem,
			Line:      pkg.Line,
			Commit:    commit,
			DepGroups: depGroups,
		})
	}

	return packages
}

type PnpmLockExtractor struct{}

func (e PnpmLockExtractor) ShouldExtract(path string) bool {
	return filepath.Base(path) == "pnpm-lock.yaml"
}

func (e PnpmLockExtractor) Extract(f DepFile) ([]PackageDetails, error) {
	var parsedLockfile *PnpmLockfile

	err := yaml.NewDecoder(f).Decode(&parsedLockfile)

	if err != nil && !errors.Is(err, io.EOF) {
		return []PackageDetails{}, fmt.Errorf("could not extract from %s: %w", f.Path(), err)
	}

	// this will happen if the file is empty
	if parsedLockfile == nil {
		parsedLockfile = &PnpmLockfile{}
	}

	return parsePnpmLock(*parsedLockfile), nil
}

var _ Extractor = PnpmLockExtractor{}

//nolint:gochecknoinits
func init() {
	registerExtractor("pnpm-lock.yaml", PnpmLockExtractor{})
}

func ParsePnpmLock(pathToLockfile string) ([]PackageDetails, error) {
	return extractFromFile(pathToLockfile, PnpmLockExtractor{})
}
