package framework

import "github.com/aws/eks-anywhere/pkg/semver"

type CommandOpt func(*string, *[]string) (err error)

func appendOpt(new ...string) CommandOpt {
	return func(binaryPath *string, args *[]string) (err error) {
		*args = append(*args, new...)
		return nil
	}
}

func withKubeconfig(kubeconfigFile string) CommandOpt {
	return appendOpt("--kubeconfig", kubeconfigFile)
}

func ExecuteWithEksaVersion(version *semver.Version, branchName string) CommandOpt {
	return func(binaryPath *string, args *[]string) (err error) {
		b, err := GetReleaseBinaryFromVersion(version)
		*binaryPath = b
		if err := setCodebuildSourceVersionEnvVar(branchName); err != nil {
			return err
		}
		return err
	}
}

func ExecuteWithLatestMinorReleaseFromVersion(version *semver.Version, branchName string) CommandOpt {
	return func(binaryPath *string, args *[]string) (err error) {
		b, err := GetLatestMinorReleaseBinaryFromVersion(version)
		*binaryPath = b
		if err := setCodebuildSourceVersionEnvVar(branchName); err != nil {
			return err
		}
		return err
	}
}

func ExecuteWithLatestMinorReleaseFromMain(branchName string) CommandOpt {
	return func(binaryPath *string, args *[]string) (err error) {
		b, err := GetLatestMinorReleaseBinaryFromMain()
		*binaryPath = b
		if err := setCodebuildSourceVersionEnvVar(branchName); err != nil {
			return err
		}
		return err
	}
}
