package workflows

import (
	"context"

	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/task"
)

type CollectDiagnosticsTask struct {
	*CollectWorkloadClusterDiagnosticsTask
	*CollectMgmtClusterDiagnosticsTask
}

func (s *CollectDiagnosticsTask) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return s.Run(ctx, commandContext), nil
}

func (s *CollectDiagnosticsTask) Checkpoint() task.TaskCheckpoint {
	return nil
}

func (s *CollectDiagnosticsTask) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

type CollectWorkloadClusterDiagnosticsTask struct{}

type CollectMgmtClusterDiagnosticsTask struct{}

func (s *CollectMgmtClusterDiagnosticsTask) Restore(ctx context.Context, commandContext *task.CommandContext, checkpoint task.UnmarshallTaskCheckpoint) (task.TaskCheckpoint, error) {
	return nil, nil
}

func (s *CollectMgmtClusterDiagnosticsTask) NextTask(commandContext *task.CommandContext) task.Task {
	return nil
}

func (s *CollectMgmtClusterDiagnosticsTask) Checkpoint() task.TaskCheckpoint {
	return nil
}

// CollectDiagnosticsTask implementation

func (s *CollectDiagnosticsTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("collecting cluster diagnostics")
	_ = s.CollectMgmtClusterDiagnosticsTask.Run(ctx, commandContext)
	_ = s.CollectWorkloadClusterDiagnosticsTask.Run(ctx, commandContext)
	return nil
}

func (s *CollectDiagnosticsTask) Name() string {
	return "collect-cluster-diagnostics"
}

// CollectWorkloadClusterDiagnosticsTask implementation

func (s *CollectWorkloadClusterDiagnosticsTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("collecting workload cluster diagnostics")
	_ = commandContext.ClusterManager.SaveLogsWorkloadCluster(ctx, commandContext.Provider, commandContext.ClusterSpec, commandContext.WorkloadCluster)
	return nil
}

func (s *CollectWorkloadClusterDiagnosticsTask) Name() string {
	return "collect-workload-cluster-diagnostics"
}

// CollectMgmtClusterDiagnosticsTask implementation

func (s *CollectMgmtClusterDiagnosticsTask) Run(ctx context.Context, commandContext *task.CommandContext) task.Task {
	logger.Info("collecting management cluster diagnostics")
	_ = commandContext.ClusterManager.SaveLogsManagementCluster(ctx, commandContext.ClusterSpec, commandContext.BootstrapCluster)
	return nil
}

func (s *CollectMgmtClusterDiagnosticsTask) Name() string {
	return "collect-management-cluster-diagnostics"
}
