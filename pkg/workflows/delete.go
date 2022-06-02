package workflows

import (
	"context"

	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/task"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
)

type Delete struct {
	bootstrapper   interfaces.Bootstrapper
	provider       providers.Provider
	clusterManager interfaces.ClusterManager
	addonManager   interfaces.AddonManager
}

func NewDelete(bootstrapper interfaces.Bootstrapper, provider providers.Provider,
	clusterManager interfaces.ClusterManager, addonManager interfaces.AddonManager,
) *Delete {
	return &Delete{
		bootstrapper:   bootstrapper,
		provider:       provider,
		clusterManager: clusterManager,
		addonManager:   addonManager,
	}
}

func (c *Delete) Run(ctx context.Context, workloadCluster *types.Cluster, clusterSpec *cluster.Spec, forceCleanup bool, kubeconfig string) error {
	if forceCleanup {
		if err := c.bootstrapper.DeleteBootstrapCluster(ctx, &types.Cluster{
			Name: workloadCluster.Name,
		}, false); err != nil {
			return err
		}
	}

	commandContext := &task.CommandContext{
		Bootstrapper:    c.bootstrapper,
		Provider:        c.provider,
		ClusterManager:  c.clusterManager,
		AddonManager:    c.addonManager,
		WorkloadCluster: workloadCluster,
		ClusterSpec:     clusterSpec,
	}

	if clusterSpec.ManagementCluster != nil {
		commandContext.BootstrapCluster = clusterSpec.ManagementCluster
	}

	return task.NewTaskRunner(&setupAndValidate{}, nil).RunTask(ctx, commandContext)
}

type setupAndValidate struct{}

func (s *setupAndValidate) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *setupAndValidate) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *setupAndValidate) Checkpoint() task.TaskCheckpoint {
	return nil
}

type createManagementCluster struct{}

func (s *createManagementCluster) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *createManagementCluster) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *createManagementCluster) Checkpoint() task.TaskCheckpoint {
	return nil
}

type installCAPI struct{}

func (s *installCAPI) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *installCAPI) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *installCAPI) Checkpoint() task.TaskCheckpoint {
	return nil
}

type moveClusterManagement struct{}

func (s *moveClusterManagement) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *moveClusterManagement) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *moveClusterManagement) Checkpoint() task.TaskCheckpoint {
	return nil
}

type deleteWorkloadCluster struct{}

func (s *deleteWorkloadCluster) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *deleteWorkloadCluster) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *deleteWorkloadCluster) Checkpoint() task.TaskCheckpoint {
	return nil
}

type cleanupGitRepo struct{}

func (s *cleanupGitRepo) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *cleanupGitRepo) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *cleanupGitRepo) Checkpoint() task.TaskCheckpoint {
	return nil
}

type deleteManagementCluster struct{}

func (s *deleteManagementCluster) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *deleteManagementCluster) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *deleteManagementCluster) Checkpoint() task.TaskCheckpoint {
	return nil
}

func (s *setupAndValidate) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Performing provider setup and validations")
	err := commandContext.Provider.SetupAndValidateDeleteCluster(ctx, commandContext.WorkloadCluster)
	if err != nil {
		commandContext.SetError(err)
		return nil
	}
	return &createManagementCluster{}
}

func (s *setupAndValidate) Name() string {
	return "setup-and-validate"
}

func (s *createManagementCluster) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	if commandContext.BootstrapCluster != nil && commandContext.BootstrapCluster.ExistingManagement {
		return &deleteWorkloadCluster{}
	}
	logger.Info("Creating management cluster")
	bootstrapOptions, err := commandContext.Provider.BootstrapClusterOpts()
	if err != nil {
		logger.Error(err, "Error getting management options from provider")
		commandContext.SetError(err)
		return nil
	}

	bootstrapCluster, err := commandContext.Bootstrapper.CreateBootstrapCluster(ctx, commandContext.ClusterSpec, bootstrapOptions...)
	if err != nil {
		commandContext.SetError(err)
		return &deleteManagementCluster{}
	}
	commandContext.BootstrapCluster = bootstrapCluster

	return &installCAPI{}
}

func (s *createManagementCluster) Name() string {
	return "management-cluster-init"
}

func (s *installCAPI) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Installing cluster-api providers on management cluster")
	err := commandContext.ClusterManager.InstallCAPI(ctx, commandContext.ClusterSpec, commandContext.BootstrapCluster, commandContext.Provider)
	if err != nil {
		commandContext.SetError(err)
		return &deleteManagementCluster{}
	}
	return &moveClusterManagement{}
}

func (s *installCAPI) Name() string {
	return "install-capi"
}

func (s *moveClusterManagement) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Moving cluster management from workload cluster")
	err := commandContext.ClusterManager.MoveCAPI(ctx, commandContext.WorkloadCluster, commandContext.BootstrapCluster, commandContext.WorkloadCluster.Name, commandContext.ClusterSpec, types.WithNodeRef())
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}
	return &deleteWorkloadCluster{}
}

func (s *moveClusterManagement) Name() string {
	return "cluster-management-move"
}

func (s *deleteWorkloadCluster) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Deleting workload cluster")
	err := commandContext.ClusterManager.DeleteCluster(ctx, commandContext.BootstrapCluster, commandContext.WorkloadCluster, commandContext.Provider, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	return &cleanupGitRepo{}
}

func (s *deleteWorkloadCluster) Name() string {
	return "delete-workload-cluster"
}

func (s *cleanupGitRepo) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("Clean up Git Repo")
	err := commandContext.AddonManager.CleanupGitRepo(ctx, commandContext.ClusterSpec)
	if err != nil {
		commandContext.SetError(err)
		return &CollectDiagnosticsTask{}
	}

	return &deleteManagementCluster{}
}

func (s *cleanupGitRepo) Name() string {
	return "clean-up-git-repo"
}

func (s *deleteManagementCluster) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	if commandContext.OriginalError != nil {
		collector := &CollectMgmtClusterDiagnosticsTask{}
		collector.Run(ctx, commandContext)
	}
	if commandContext.BootstrapCluster != nil && !commandContext.BootstrapCluster.ExistingManagement {
		if err := commandContext.Bootstrapper.DeleteBootstrapCluster(ctx, commandContext.BootstrapCluster, false); err != nil {
			commandContext.SetError(err)
		}
		return nil
	}
	logger.Info("Bootstrap cluster information missing - skipping delete kind cluster")
	if commandContext.OriginalError == nil {
		logger.MarkSuccess("Cluster deleted!")
	}
	return nil
}

func (s *deleteManagementCluster) Name() string {
	return "kind-cluster-delete"
}
