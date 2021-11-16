// +build e2e

package e2e

import (
	"bytes"
	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/files"
	"github.com/aws/eks-anywhere/pkg/semver"
	"github.com/aws/eks-anywhere/pkg/validations"
	releasev1alpha1 "github.com/aws/eks-anywhere/release/api/v1alpha1"
	"github.com/aws/eks-anywhere/test/framework"
	"io"
	"os"
	"path/filepath"
)

const (
	prodReleasesManifest = "https://anywhere-assets.eks.amazonaws.com/releases/eks-a/manifest.yaml"
	releaseBinaryName    = "eksctl-anywhere"
)

func getLatestMinorReleaseFromMain(test *framework.ClusterE2ETest) *releasev1alpha1.EksARelease {
	reader := cluster.NewManifestReader()
	test.T.Logf("Reading prod release manifest %s", prodReleasesManifest)
	releases, err := reader.GetReleases(prodReleasesManifest)
	if err != nil {
		test.T.Fatal(err)
	}

	var latestRelease *releasev1alpha1.EksARelease
	for _, release := range releases.Spec.Releases {
		if release.Version == releases.Spec.LatestVersion {
			latestRelease = &release
			break
		}
	}

	if latestRelease == nil {
		test.T.Fatalf("Releases manifest doesn't contain latest release %s", releases.Spec.LatestVersion)
	}

	return latestRelease
}

func getLatestMinorReleaseFromReleaseBranch(test *framework.ClusterE2ETest, releaseBranchVersion *semver.Version) *releasev1alpha1.EksARelease {
	reader := cluster.NewManifestReader()
	test.T.Logf("Reading prod release manifest %s", prodReleasesManifest)
	releases, err := reader.GetReleases(prodReleasesManifest)
	if err != nil {
		test.T.Fatal(err)
	}

	var latestPrevMinorRelease *releasev1alpha1.EksARelease
	latestPrevMinorReleaseVersion, err := semver.New("0.0.0")
	if err != nil {
		test.T.Fatal(err)
	}

	for _, release := range releases.Spec.Releases {
		releaseVersion, err := semver.New(release.Version)
		if err != nil {
			test.T.Fatal(err)
		}
		if releaseVersion.LessThan(releaseBranchVersion) && releaseVersion.Minor != releaseBranchVersion.Minor && releaseVersion.GreaterThan(latestPrevMinorReleaseVersion) {
			latestPrevMinorRelease = &release
			latestPrevMinorReleaseVersion, err = semver.New(release.Version)
			if err != nil {
				test.T.Fatal(err)
			}
		}
	}

	if latestPrevMinorRelease == nil {
		test.T.Fatalf("Releases manifest doesn't contain a version of the previous minor release")
	}

	return latestPrevMinorRelease
}

func getBinary(test *framework.ClusterE2ETest, release releasev1alpha1.EksARelease) string {
	latestReleaseBinaryFolder := filepath.Join("bin", release.Version)
	latestReleaseBinaryPath := filepath.Join(latestReleaseBinaryFolder, releaseBinaryName)

	if !validations.FileExists(latestReleaseBinaryPath) {
		test.T.Logf("Reading prod latest release tarball %s", release.EksABinary.LinuxBinary.URI)
		reader := cluster.NewManifestReader()
		latestReleaseTar, err := reader.ReadFile(release.EksABinary.LinuxBinary.URI)
		if err != nil {
			test.T.Fatalf("Failed downloading tar for latest release: %s", err)
		}

		test.T.Log("Unpacking the release tarball")

		err = unpackTarball(latestReleaseBinaryFolder, bytes.NewReader(latestReleaseTar))
		if err != nil {
			test.T.Fatalf("Failed unpacking artifacts for latest release: %s", err)
		}
	}

	return latestReleaseBinaryPath
}

func unpackTarball(destinationFolder string, r io.Reader) error {
	err := os.MkdirAll(destinationFolder, os.ModePerm)
	if err != nil {
		return err
	}

	err = files.SetupBinary(r, destinationFolder, releaseBinaryName)
	if err != nil {
		return err
	}
	return nil
}
