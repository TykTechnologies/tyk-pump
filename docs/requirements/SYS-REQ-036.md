# SYS-REQ-036: FIPS release artifacts preserve the FIPS build variant

## Intent
FIPS-labelled release artifacts shall preserve the FIPS build variant. A
`tyk-pump-fips` package must be assembled only from FIPS build IDs, and a FIPS
Docker image must install the `tyk-pump-fips` package and use a FIPS base image.
This complements **SYS-REQ-026**, which covers local FIPS build availability via
`make build-fips`; SYS-REQ-036 covers distribution wiring.

## Motivation
Operators in regulated environments select artifacts by package name, Docker
repository, and tag. If an image or package is labelled FIPS while it contains
the standard binary/package, the deployment is non-compliant under a
compliant-looking name. That failure mode is release-pipeline-specific: the
runtime pump cannot detect it after the wrong artifact has already been built.

## Formalization
```
release shall always satisfy fips_release_artifact_variant_preserved
```
The truth condition is static over the release configuration: every FIPS package
entry points at FIPS build IDs, every FIPS Docker-image step installs the FIPS
package name, and the FIPS Docker-image step uses a FIPS base image.

## Code References
- `ci/goreleaser/goreleaser.yml` — defines FIPS build IDs and the `tyk-pump-fips`
  package.
- `.github/workflows/release.yml` — builds FIPS Docker images from the
  `tyk-pump-fips` package and FIPS base image.
- `ci/Dockerfile.distroless` — installs the package named by
  `BUILD_PACKAGE_NAME`.

## Evidence
- `build_fips_invariant_test.go:TestReleaseInvariant_FIPSArtifactsUseFIPSBuilds_SYSREQ036`
  parses the release configuration and proves FIPS packages/images are wired to
  FIPS variants.
- `DEFECT-48` records the historical `206c1d0` mismatch where FIPS-labelled
  Docker images used the standard build ID.

## Open Questions
- This static witness checks release wiring, not a pushed image digest. A
  downstream provenance/SBOM attestation check could prove the same invariant on
  the produced artifact after CI builds it.
