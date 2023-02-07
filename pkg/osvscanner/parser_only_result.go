package osvscanner

import (
	"fmt"

	"github.com/google/osv-scanner/internal/output"
	"github.com/google/osv-scanner/pkg/grouper"
	"github.com/google/osv-scanner/pkg/models"
	"github.com/google/osv-scanner/pkg/osv"
)

// groupBySource converts parsed information
// grouped by source location.
func groupBySource(r *output.Reporter, query osv.BatchedQuery) models.VulnerabilityResults {
	output := models.VulnerabilityResults{
		Results: []models.PackageSource{},
	}
	groupedBySource := map[models.SourceInfo][]models.PackageVulns{}

	for _, query := range query.Queries {
		var pkg models.PackageVulns
		if query.Commit != "" {
			pkg.Package.Version = query.Commit
			pkg.Package.Ecosystem = "GIT"
		} else if query.Package.PURL != "" {
			var err error
			pkg.Package, err = PURLToPackage(query.Package.PURL)
			if err != nil {
				r.PrintError(fmt.Sprintf("Failed to parse purl: %s, with error: %s",
					query.Package.PURL, err))

				continue
			}
		} else {
			pkg = models.PackageVulns{
				Package: models.PackageInfo{
					Name:      query.Package.Name,
					Version:   query.Version,
					Ecosystem: query.Package.Ecosystem,
				},
			}
		}

		pkg.Vulnerabilities = nil

		pkg.Groups = grouper.Group(grouper.ConvertVulnerabilityToIDAliases(pkg.Vulnerabilities))
		groupedBySource[query.Source] = append(groupedBySource[query.Source], pkg)
	}

	for source, packages := range groupedBySource {
		output.Results = append(output.Results, models.PackageSource{
			Source:   source,
			Packages: packages,
		})
	}

	return output
}
