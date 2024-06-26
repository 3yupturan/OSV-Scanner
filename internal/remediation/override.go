package remediation

import (
	"context"
	"errors"
	"slices"

	"deps.dev/util/resolve"
	"deps.dev/util/resolve/dep"
	"deps.dev/util/semver"
	"github.com/google/osv-scanner/internal/resolution"
	"github.com/google/osv-scanner/internal/resolution/client"
	"github.com/google/osv-scanner/internal/resolution/manifest"
	"github.com/google/osv-scanner/internal/resolution/util"
	"github.com/google/osv-scanner/internal/utility/vulns"
)

type overridePatch struct {
	resolve.PackageKey
	OrigVersion string
	NewVersion  string
}

func ComputeOverridePatches(ctx context.Context, cl client.ResolutionClient, result *resolution.ResolutionResult, opts RemediationOptions) ([]resolution.ResolutionDiff, error) {
	// TODO: this is very similar to ComputeRelaxPatches - can the common parts be factored out?
	// Filter the original result just in case it hasn't been already
	result.FilterVulns(opts.MatchVuln)

	// Do the resolutions concurrently
	type overrideResult struct {
		vulnIDs []string
		result  *resolution.ResolutionResult
		patches []overridePatch
		err     error
	}
	ch := make(chan overrideResult)
	doOverride := func(vulnIDs []string) {
		res, patches, err := overridePatchVulns(ctx, cl, result, vulnIDs, opts)
		if err == nil {
			res.FilterVulns(opts.MatchVuln)
		}
		ch <- overrideResult{
			vulnIDs: vulnIDs,
			result:  res,
			patches: patches,
			err:     err,
		}
	}

	toProcess := 0
	for _, v := range result.Vulns {
		// TODO: limit the number of goroutines
		go doOverride([]string{v.Vulnerability.ID})
		toProcess++
	}

	var allResults []resolution.ResolutionDiff
	for toProcess > 0 {
		res := <-ch
		toProcess--
		if errors.Is(res.err, errOverrideImpossible) {
			continue
		}

		if res.err != nil {
			// TODO: stop goroutines
			return nil, res.err
		}

		diff := result.CalculateDiff(res.result)

		// CalculateDiff does not compute override manifest patches correctly, manually fill it out.
		// TODO: CalculateDiff maybe should not be reconstructing patches.
		// Refactor CalculateDiff, Relaxer, Override to make patches in a more sane way.
		diff.Deps = make([]manifest.DependencyPatch, len(res.patches))
		for i, p := range res.patches {
			diff.Deps[i] = manifest.DependencyPatch{
				Pkg:          p.PackageKey,
				Type:         dep.Type{},
				OrigRequire:  "", // Using empty original to signal this is an override patch
				OrigResolved: p.OrigVersion,
				NewRequire:   p.NewVersion,
				NewResolved:  p.NewVersion,
			}
		}

		allResults = append(allResults, diff)

		// If there are any new vulns, try override them as well
		var newlyAdded []string
		for _, v := range diff.AddedVulns {
			if !slices.Contains(res.vulnIDs, v.Vulnerability.ID) {
				newlyAdded = append(newlyAdded, v.Vulnerability.ID)
			}
		}

		if len(newlyAdded) > 0 {
			go doOverride(append(res.vulnIDs, newlyAdded...)) // No need to clone res.vulnIDs here
			toProcess++
		}
	}

	// Sort and remove duplicate patches
	slices.SortFunc(allResults, func(a, b resolution.ResolutionDiff) int { return a.Compare(b) })
	allResults = slices.CompactFunc(allResults, func(a, b resolution.ResolutionDiff) bool { return a.Compare(b) == 0 })

	return allResults, nil
}

var errOverrideImpossible = errors.New("cannot fix vulns by overrides")

func overridePatchVulns(ctx context.Context, cl client.ResolutionClient, result *resolution.ResolutionResult, vulnIDs []string, opts RemediationOptions) (*resolution.ResolutionResult, []overridePatch, error) {
	// Try to fix as many vulns in vulnIDs as possible.
	// returns errOverrideImpossible if 0 vulns are patchable.
	var effectivePatches []overridePatch
	for {
		// Find the relevant vulns affecting each version key.
		vkVulns := make(map[resolve.VersionKey][]*resolution.ResolutionVuln)
		for i, v := range result.Vulns {
			if !slices.Contains(vulnIDs, v.Vulnerability.ID) {
				continue
			}
			seenVks := make(map[resolve.VersionKey]struct{})
			for _, c := range v.ProblemChains {
				vk, _ := c.End()
				if _, seen := seenVks[vk]; !seen {
					vkVulns[vk] = append(vkVulns[vk], &result.Vulns[i])
					seenVks[vk] = struct{}{}
				}
			}
			for _, c := range v.NonProblemChains {
				vk, _ := c.End()
				if _, seen := seenVks[vk]; !seen {
					vkVulns[vk] = append(vkVulns[vk], &result.Vulns[i])
					seenVks[vk] = struct{}{}
				}
			}
		}

		if len(vkVulns) == 0 {
			// All vulns have been fixed.
			break
		}

		newPatches := make([]overridePatch, 0, len(vkVulns))

		for vk, vulnerabilities := range vkVulns {
			// Consider vulns affecting packages we don't want to change unfixable
			if slices.Contains(opts.AvoidPkgs, vk.Name) {
				continue
			}

			sys := vk.Semver()
			// Get & sort all the valid versions of this package
			// TODO: (Maven) skip unlisted versions and versions on other registries
			versions, err := cl.Versions(ctx, vk.PackageKey)
			if err != nil {
				return nil, nil, err
			}
			cmpFunc := func(a, b resolve.Version) int { return sys.Compare(a.Version, b.Version) }
			slices.SortFunc(versions, cmpFunc)
			startIdx, vkFound := slices.BinarySearchFunc(versions, resolve.Version{VersionKey: vk}, cmpFunc)
			if vkFound {
				startIdx++
			}

			bestVK := vk
			bestCount := len(vulnerabilities) // remaining vulns

			// Find the minimal greater version that fixes as many vulnerabilities as possible.
			for _, ver := range versions[startIdx:] {
				if !opts.AllowMajor {
					if _, diff, _ := sys.Difference(vk.Version, ver.Version); diff == semver.DiffMajor {
						break
					}
				}

				count := 0 // remaining vulns
				for _, rv := range vulnerabilities {
					if vulns.IsAffected(rv.Vulnerability, util.VKToPackageDetails(ver.VersionKey)) {
						count += 1
					}
				}

				if count < bestCount {
					bestCount = count
					bestVK = ver.VersionKey

					if bestCount == 0 { // stop if there are 0 vulns remaining
						break
					}
				}
			}
			if bestCount < len(vulnerabilities) {
				newPatches = append(newPatches, overridePatch{
					PackageKey:  vk.PackageKey,
					OrigVersion: vk.Version,
					NewVersion:  bestVK.Version,
				})
			}
		}

		if len(newPatches) == 0 {
			break
		}

		// Patch and re-resolve manifest
		newManif, err := patchManifest(newPatches, result.Manifest)
		if err != nil {
			return nil, nil, err
		}

		result, err = resolution.Resolve(ctx, cl, newManif)
		if err != nil {
			return nil, nil, err
		}

		result.FilterVulns(opts.MatchVuln)

		// If the patch applies to a package that was already patched before, update the effective patch.
		for _, p := range newPatches {
			idx := slices.IndexFunc(effectivePatches, func(op overridePatch) bool { return op.PackageKey == p.PackageKey && op.NewVersion == p.OrigVersion })
			if idx == -1 {
				effectivePatches = append(effectivePatches, p)
			} else {
				effectivePatches[idx].NewVersion = p.NewVersion
			}
		}
	}

	if len(effectivePatches) == 0 {
		return nil, nil, errOverrideImpossible
	}

	slices.SortFunc(effectivePatches, func(a, b overridePatch) int {
		if c := a.PackageKey.Compare(b.PackageKey); c != 0 {
			return c
		}
		return a.Semver().Compare(a.OrigVersion, b.OrigVersion)
	})

	return result, effectivePatches, nil
}

func patchManifest(patches []overridePatch, m manifest.Manifest) (manifest.Manifest, error) {
	if m.System() != resolve.Maven {
		return manifest.Manifest{}, errors.New("unsupported ecosystem")
	}

	// TODO: may need special handling for the artifact's type and classifier

	patched := m.Clone()

	for _, p := range patches {
		found := false
		i := 0
		for _, r := range patched.Requirements {
			if r.PackageKey != p.PackageKey {
				patched.Requirements[i] = r
				i++

				continue
			}
			if origin, hasOrigin := r.Type.GetAttr(dep.MavenDependencyOrigin); !hasOrigin || origin == "management" {
				found = true
				r.Version = p.NewVersion
				patched.Requirements[i] = r
				i++
			}
		}
		patched.Requirements = patched.Requirements[:i]
		if !found {
			newReq := resolve.RequirementVersion{
				VersionKey: resolve.VersionKey{
					PackageKey:  p.PackageKey,
					Version:     p.NewVersion,
					VersionType: resolve.Requirement,
				},
			}
			newReq.Type.AddAttr(dep.MavenDependencyOrigin, "management")
			patched.Requirements = append(patched.Requirements, newReq)
		}
	}

	return patched, nil
}