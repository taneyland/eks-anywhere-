package workflows

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/clustermarshaller"
	"github.com/aws/eks-anywhere/pkg/filewriter"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/task"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
	"os"
)

type Upgrade struct {
	bootstrapper      interfaces.Bootstrapper
	provider          providers.Provider
	clusterManager    interfaces.ClusterManager
	addonManager      interfaces.AddonManager
	writer            filewriter.FileWriter
	capiManager       interfaces.CAPIManager
	eksdInstaller     interfaces.EksdInstaller
	eksdUpgrader      interfaces.EksdUpgrader
	upgradeChangeDiff *types.ChangeDiff
}

func NewUpgrade(bootstrapper interfaces.Bootstrapper, provider providers.Provider,
	capiManager interfaces.CAPIManager,
	clusterManager interfaces.ClusterManager, addonManager interfaces.AddonManager, writer filewriter.FileWriter, eksdUpgrader interfaces.EksdUpgrader, eksdInstaller interfaces.EksdInstaller,
) *Upgrade {
	upgradeChangeDiff := types.NewChangeDiff()
	return &Upgrade{
		bootstrapper:      bootstrapper,
		provider:          provider,
		clusterManager:    clusterManager,
		addonManager:      addonManager,
		writer:            writer,
		capiManager:       capiManager,
		eksdUpgrader:      eksdUpgrader,
		eksdInstaller:     eksdInstaller,
		upgradeChangeDiff: upgradeChangeDiff,
	}
}

func (c *Upgrade) Run(ctx context.Context, clusterSpec *cluster.Spec, managementCluster *types.Cluster, workloadCluster *types.Cluster, validator interfaces.Validator, forceCleanup bool) error {
	if forceCleanup {
		if err := c.bootstrapper.DeleteBootstrapCluster(ctx, &types.Cluster{
			Name: clusterSpec.Cluster.Name,
		}, true); err != nil {
			return err
		}
	}

	commandContext := &task.CommandContext{
		Bootstrapper:      c.bootstrapper,
		Provider:          c.provider,
		ClusterManager:    c.clusterManager,
		AddonManager:      c.addonManager,
		ManagementCluster: managementCluster,
		WorkloadCluster:   workloadCluster,
		ClusterSpec:       clusterSpec,
		Validations:       validator,
		Writer:            c.writer,
		CAPIManager:       c.capiManager,
		EksdInstaller:     c.eksdInstaller,
		EksdUpgrader:      c.eksdUpgrader,
		UpgradeChangeDiff: c.upgradeChangeDiff,
	}

	return task.NewTaskRunner(&setupAndValidateTasks{}, c.writer).RunTask(ctx, commandContext)
}

type setupAndValidateTasks struct {
	writer filewriter.FileWriter
}

type updateSecrets struct{}

type ensureEtcdCAPIComponentsExistTask struct{}

type upgradeCoreComponents struct{}

type upgradeNeeded struct{}

type pauseEksaAndFluxReconcile struct{}

type createBootstrapClusterTask struct{}

type installCAPITask struct{}

type moveManagementToBootstrapTask struct{}

func (s *moveManagementToBootstrapTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

type moveManagementToWorkloadTaskAndExit struct {
	*moveManagementToWorkloadTask
}

type moveManagementToWorkloadTask struct{}

type upgradeWorkloadClusterTask struct{}

type deleteBootstrapClusterTask struct {
	*CollectDiagnosticsTask
}

type updateClusterAndGitResources struct{}

type resumeFluxReconcile struct{}

type writeClusterConfigTask struct{}

func (s *setupAndValidateTasks) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Performing setup and validations")

	runner := validations.NewRunner()
	runner.Register(s.validations(ctx, commandContext)...)

	err := runner.Run()
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	return &updateSecrets{}
}

func (s *setupAndValidateTasks) validations(ctx context.Context, commandContext *task.CommandContext) []validations.Validation {
	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: fmt.Sprintf("%s Provider setup is valid", commandContext.Provider.Name()),
				Err:  commandContext.Provider.SetupAndValidateUpgradeCluster(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec),
			}
		},
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name: "upgrade preflight validations pass",
				Err:  commandContext.Validations.PreflightValidations(ctx),
			}
		},
	}
}

func (s *setupAndValidateTasks) Name() string {
	return "setup-and-validate"
}

func (s *setupAndValidateTasks) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *setupAndValidateTasks) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &updateSecrets{}
}

func (s *updateSecrets) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	err := commandContext.Provider.UpdateSecrets(ctx, commandContext.ManagementCluster)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	// fake error
	if os.Getenv("FAKE_ERROR1") == "true" {
		commandContext.SetError(errors.New("fake error 1 in upgrade"))
		return nil
	}
	return &ensureEtcdCAPIComponentsExistTask{}
}

func (s *updateSecrets) Name() string {
	return "update-secrets"
}

func (s *updateSecrets) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *updateSecrets) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &ensureEtcdCAPIComponentsExistTask{}
}

func (s *ensureEtcdCAPIComponentsExistTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Ensuring etcd CAPI providers exist on management cluster before upgrade")
	currentSpec, err := commandContext.ClusterManager.GetCurrentClusterSpec(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec.Cluster.Name)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.CurrentClusterSpec = currentSpec
	if err := commandContext.CAPIManager.EnsureEtcdProvidersInstallation(ctx, commandContext.ManagementCluster, commandContext.Provider, currentSpec); err != nil {
		commandContext.SetError(err)
		return nil
	}
	// fake error
	if os.Getenv("FAKE_ERROR2") == "true" {
		commandContext.SetError(errors.New("fake error 2 in upgrade"))
		return nil
	}
	return &upgradeCoreComponents{}
}

func (s *ensureEtcdCAPIComponentsExistTask) Name() string {
	return "ensure-etcd-capi-components-exist"
}

func (s *ensureEtcdCAPIComponentsExistTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *ensureEtcdCAPIComponentsExistTask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &upgradeCoreComponents{}
}

func (s *upgradeCoreComponents) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Upgrading core components")

	changeDiff, err := commandContext.ClusterManager.UpgradeNetworking(ctx, commandContext.WorkloadCluster, commandContext.CurrentClusterSpec, commandContext.ClusterSpec, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.UpgradeChangeDiff.Append(changeDiff)

	changeDiff, err = commandContext.CAPIManager.Upgrade(ctx, commandContext.ManagementCluster, commandContext.Provider, commandContext.CurrentClusterSpec, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.UpgradeChangeDiff.Append(changeDiff)

	err = commandContext.AddonManager.UpdateLegacyFileStructure(ctx, commandContext.CurrentClusterSpec, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	changeDiff, err = commandContext.AddonManager.Upgrade(ctx, commandContext.ManagementCluster, commandContext.CurrentClusterSpec, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.UpgradeChangeDiff.Append(changeDiff)

	changeDiff, err = commandContext.ClusterManager.Upgrade(ctx, commandContext.ManagementCluster, commandContext.CurrentClusterSpec, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.UpgradeChangeDiff.Append(changeDiff)

	changeDiff, err = commandContext.EksdUpgrader.Upgrade(ctx, commandContext.ManagementCluster, commandContext.CurrentClusterSpec, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.UpgradeChangeDiff.Append(changeDiff)
	return &upgradeNeeded{}
}

func (s *upgradeCoreComponents) Name() string {
	return "upgrade-core-components"
}

func (s *upgradeCoreComponents) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *upgradeCoreComponents) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &upgradeNeeded{}
}

func (s *upgradeNeeded) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	newSpec := commandContext.ClusterSpec

	if upgradeNeeded, err := commandContext.Provider.UpgradeNeeded(ctx, newSpec, commandContext.CurrentClusterSpec, commandContext.ManagementCluster); err != nil {
		commandContext.SetError(err)
		return nil
	} else if upgradeNeeded {
		logger.V(3).Info("Provider needs a cluster upgrade")
		return &pauseEksaAndFluxReconcile{}
	}
	diff, err := commandContext.ClusterManager.EKSAClusterSpecChanged(ctx, commandContext.ManagementCluster, newSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	if !diff {
		logger.Info("No upgrades needed from cluster spec")
		return nil
	}
	return &pauseEksaAndFluxReconcile{}
}

func (s *upgradeNeeded) Name() string {
	return "upgrade-needed"
}

func (s *upgradeNeeded) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *upgradeNeeded) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &pauseEksaAndFluxReconcile{}
}

func (s *pauseEksaAndFluxReconcile) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Pausing EKS-A cluster controller reconcile")
	err := commandContext.ClusterManager.PauseEKSAControllerReconcile(ctx, commandContext.ManagementCluster, commandContext.CurrentClusterSpec, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	logger.Info("Pausing Flux kustomization")
	err = commandContext.AddonManager.PauseGitOpsKustomization(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	return &createBootstrapClusterTask{}
}

func (s *pauseEksaAndFluxReconcile) Name() string {
	return "pause-controllers-reconcile"
}

func (s *pauseEksaAndFluxReconcile) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *pauseEksaAndFluxReconcile) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &createBootstrapClusterTask{}
}

func (s *createBootstrapClusterTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	if commandContext.ManagementCluster != nil && commandContext.ManagementCluster.ExistingManagement {
		return &upgradeWorkloadClusterTask{}
	}

	logger.Info("Creating bootstrap cluster")
	bootstrapOptions, err := commandContext.Provider.BootstrapClusterOpts()
	if err != nil {
		commandContext.SetError(err)
		return nil
	}

	bootstrapCluster, err := commandContext.Bootstrapper.CreateBootstrapCluster(ctx, commandContext.ClusterSpec, bootstrapOptions...)
	commandContext.BootstrapCluster = bootstrapCluster
	if err != nil {
		commandContext.SetError(err)
		return &deleteBootstrapClusterTask{}
	}

	return &installCAPITask{}
}

func (s *createBootstrapClusterTask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	if commandContext.ManagementCluster != nil && commandContext.ManagementCluster.ExistingManagement {
		return &upgradeWorkloadClusterTask{}
	}
	return &installCAPITask{}
}

func (s *createBootstrapClusterTask) Name() string {
	return "bootstrap-cluster-init"
}

func (s *createBootstrapClusterTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *installCAPITask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Installing cluster-api providers on bootstrap cluster")
	err := commandContext.ClusterManager.InstallCAPI(ctx, commandContext.ClusterSpec, commandContext.BootstrapCluster, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		return &deleteBootstrapClusterTask{}
	}
	return &moveManagementToBootstrapTask{}
}

func (s *installCAPITask) Name() string {
	return "install-capi"
}

func (s *installCAPITask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *installCAPITask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &moveManagementToBootstrapTask{}
}

func (s *moveManagementToBootstrapTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Moving cluster management from workload to bootstrap cluster")
	err := commandContext.ClusterManager.MoveCAPI(ctx, commandContext.WorkloadCluster, commandContext.BootstrapCluster, commandContext.WorkloadCluster.Name, commandContext.ClusterSpec, types.WithNodeRef(), types.WithNodeHealthy())
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.ManagementCluster = commandContext.BootstrapCluster
	return &upgradeWorkloadClusterTask{}
}

func (s *moveManagementToBootstrapTask) Name() string {
	return "capi-management-move-to-bootstrap"
}

func (s *moveManagementToBootstrapTask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &upgradeWorkloadClusterTask{}
}

func (s *upgradeWorkloadClusterTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	eksaManagementCluster := commandContext.WorkloadCluster
	if commandContext.ManagementCluster != nil && commandContext.ManagementCluster.ExistingManagement {
		eksaManagementCluster = commandContext.ManagementCluster
	}

	logger.Info("Upgrading workload cluster")
	err := commandContext.ClusterManager.UpgradeCluster(ctx, commandContext.ManagementCluster, commandContext.WorkloadCluster, commandContext.ClusterSpec, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		if commandContext.ManagementCluster.ExistingManagement {
			return &CollectDiagnosticsTask{}
		}
		return &moveManagementToWorkloadTaskAndExit{}
	}

	// fake error
	if os.Getenv("FAKE_ERROR") == "true" {
		commandContext.SetError(errors.New("fake error in upgrade"))
		return nil
	}

	if commandContext.UpgradeChangeDiff.Changed() {
		if err = commandContext.ClusterManager.ApplyBundles(ctx, commandContext.ClusterSpec, eksaManagementCluster); err != nil {
			commandContext.SetError(err)
			return &CollectDiagnosticsTask{}
		}
	}

	return &moveManagementToWorkloadTask{}
}

func (s *upgradeWorkloadClusterTask) Name() string {
	return "upgrade-workload-cluster"
}

func (s *upgradeWorkloadClusterTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *upgradeWorkloadClusterTask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *moveManagementToWorkloadTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	if commandContext.ManagementCluster.ExistingManagement {
		return &updateClusterAndGitResources{}
	}
	logger.Info("Moving cluster management from bootstrap to workload cluster")
	err := commandContext.ClusterManager.MoveCAPI(ctx, commandContext.BootstrapCluster, commandContext.WorkloadCluster, commandContext.WorkloadCluster.Name, commandContext.ClusterSpec, types.WithNodeRef(), types.WithNodeHealthy())
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	commandContext.ManagementCluster = commandContext.WorkloadCluster
	return &updateClusterAndGitResources{}
}

func (s *moveManagementToWorkloadTaskAndExit) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	_ = s.moveManagementToWorkloadTask.Run(ctx, commandContext)
	return &CollectDiagnosticsTask{}
}

func (s *moveManagementToWorkloadTask) Name() string {
	return "capi-management-move-to-workload"
}

func (s *moveManagementToWorkloadTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *moveManagementToWorkloadTask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &updateClusterAndGitResources{}
}

func (s *updateClusterAndGitResources) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Applying new EKS-A cluster resource; resuming reconcile")
	datacenterConfig := commandContext.Provider.DatacenterConfig(commandContext.ClusterSpec)
	machineConfigs := commandContext.Provider.MachineConfigs(commandContext.ClusterSpec)
	err := commandContext.ClusterManager.CreateEKSAResources(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec, datacenterConfig, machineConfigs)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	err = commandContext.EksdInstaller.InstallEksdManifest(ctx, commandContext.ClusterSpec, commandContext.ManagementCluster)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	logger.Info("Resuming EKS-A controller reconciliation")
	err = commandContext.ClusterManager.ResumeEKSAControllerReconcile(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	logger.Info("Updating Git Repo with new EKS-A cluster spec")
	err = commandContext.AddonManager.UpdateGitEksaSpec(ctx, commandContext.ClusterSpec, datacenterConfig, machineConfigs)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	return &resumeFluxReconcile{}
}

func (s *updateClusterAndGitResources) Name() string {
	return "update-resources"
}

func (s *updateClusterAndGitResources) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *updateClusterAndGitResources) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &resumeFluxReconcile{}
}

func (s *resumeFluxReconcile) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Forcing reconcile Git repo with latest commit")
	err := commandContext.AddonManager.ForceReconcileGitRepo(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	logger.Info("Resuming Flux kustomization")
	err = commandContext.AddonManager.ResumeGitOpsKustomization(ctx, commandContext.ManagementCluster, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &writeClusterConfigTask{}
	}
	return &writeClusterConfigTask{}
}

func (s *resumeFluxReconcile) Name() string {
	return "resume-flux-kustomization"
}

func (s *resumeFluxReconcile) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *resumeFluxReconcile) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &writeClusterConfigTask{}
}

func (s *writeClusterConfigTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Writing cluster config file")
	err := clustermarshaller.WriteClusterConfig(commandContext.ClusterSpec, commandContext.Provider.DatacenterConfig(commandContext.ClusterSpec), commandContext.Provider.MachineConfigs(commandContext.ClusterSpec), commandContext.Writer)
	if err != nil {
		commandContext.SetError(err)
	}
	return &deleteBootstrapClusterTask{}
}

func (s *writeClusterConfigTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}

func (s *writeClusterConfigTask) Name() string {
	return "write-cluster-config"
}

func (s *writeClusterConfigTask) NextTaskAfterSuccess(commandContext *task.CommandContext) task.Task {
	return &deleteBootstrapClusterTask{}
}

func (s *deleteBootstrapClusterTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	if commandContext.OriginalError != nil {
		c := CollectDiagnosticsTask{}
		c.Run(ctx, commandContext)
	}
	if commandContext.BootstrapCluster != nil && !commandContext.BootstrapCluster.ExistingManagement {
		if err := commandContext.Bootstrapper.DeleteBootstrapCluster(ctx, commandContext.BootstrapCluster, true); err != nil {
			commandContext.SetError(err)
		}
		if commandContext.OriginalError == nil {
			//DELETE CHECKPOINT FILE
			logger.MarkSuccess("Cluster upgraded!")
		}
		return nil
	}
	logger.Info("Bootstrap cluster information missing - skipping delete kind cluster")
	if commandContext.OriginalError == nil {
		//DELETE CHECKPOINT FILE
		logger.MarkSuccess("Cluster upgraded!")
	}
	return nil
}

func (s *deleteBootstrapClusterTask) Name() string {
	return "delete-kind-cluster"
}

func (s *deleteBootstrapClusterTask) Checkpoint(nextTask task.Task) task.TaskCheckpoint {
	return task.TaskCheckpoint{}
}
