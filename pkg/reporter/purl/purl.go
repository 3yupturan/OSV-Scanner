package purl

import (
	"github.com/google/osv-scanner/pkg/models"
	"github.com/package-url/packageurl-go"
)

type ParameterExtractor func(packageInfo models.PackageInfo) (namespace string, name string, ok bool)

var ecosystemToPurlMapper = map[models.Ecosystem]string{
	models.EcosystemMaven:     packageurl.TypeMaven,
	models.EcosystemGo:        packageurl.TypeGolang,
	models.EcosystemPackagist: packageurl.TypeComposer,
	models.EcosystemPyPI:      packageurl.TypePyPi,
	models.EcosystemRubyGems:  packageurl.TypeGem,
	models.EcosystemNuGet:     packageurl.TypeNuget,
	models.EcosystemNPM:       packageurl.TypeNPM,
}

var ecosystemPURLExtractor = map[models.Ecosystem]ParameterExtractor{
	models.EcosystemMaven:     extractPURLFromMaven,
	models.EcosystemGo:        ExtractPURLFromGolang,
	models.EcosystemPackagist: extractPURLFromComposer,
}

func From(packageInfo models.PackageInfo) *packageurl.PackageURL {
	var namespace string
	var name string
	version := packageInfo.Version
	ecosystem := models.Ecosystem(packageInfo.Ecosystem)
	purlType, typeExists := ecosystemToPurlMapper[ecosystem]
	parameterExtractor, extractorExists := ecosystemPURLExtractor[ecosystem]

	if !typeExists {
		return nil
	}

	if extractorExists {
		var ok bool
		namespace, name, ok = parameterExtractor(packageInfo)
		if !ok {
			return nil
		}
	} else {
		name = packageInfo.Name
	}
	return packageurl.NewPackageURL(purlType, namespace, name, version, nil, "")
}
