package task

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"time"

	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/filewriter"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/workflows/interfaces"
)

// Task is a logical unit of work - meant to be implemented by each Task
type Task interface {
	Run(ctx context.Context, commandContext *CommandContext) Task
	Name() string
	Checkpoint(nextTask Task) TaskCheckpoint
}

// Command context maintains the mutable and shared entities
type CommandContext struct {
	Bootstrapper       interfaces.Bootstrapper
	Provider           providers.Provider
	ClusterManager     interfaces.ClusterManager
	AddonManager       interfaces.AddonManager
	Validations        interfaces.Validator
	Writer             filewriter.FileWriter
	EksdInstaller      interfaces.EksdInstaller
	EksdUpgrader       interfaces.EksdUpgrader
	CAPIManager        interfaces.CAPIManager
	ClusterSpec        *cluster.Spec
	CurrentClusterSpec *cluster.Spec
	UpgradeChangeDiff  *types.ChangeDiff
	BootstrapCluster   *types.Cluster
	ManagementCluster  *types.Cluster
	WorkloadCluster    *types.Cluster
	Profiler           *Profiler
	OriginalError      error
}

func (c *CommandContext) SetError(err error) {
	if c.OriginalError == nil {
		c.OriginalError = err
	}
}

type Profiler struct {
	metrics map[string]map[string]time.Duration
	starts  map[string]map[string]time.Time
}

// profiler for a Task
func (pp *Profiler) SetStartTask(taskName string) {
	pp.SetStart(taskName, taskName)
}

// this can be used to profile sub tasks
func (pp *Profiler) SetStart(taskName string, msg string) {
	if _, ok := pp.starts[taskName]; !ok {
		pp.starts[taskName] = map[string]time.Time{}
	}
	pp.starts[taskName][msg] = time.Now()
}

// needs to be called after setStart
func (pp *Profiler) MarkDoneTask(taskName string) {
	pp.MarkDone(taskName, taskName)
}

// this can be used to profile sub tasks
func (pp *Profiler) MarkDone(taskName string, msg string) {
	if _, ok := pp.metrics[taskName]; !ok {
		pp.metrics[taskName] = map[string]time.Duration{}
	}
	if start, ok := pp.starts[taskName][msg]; ok {
		pp.metrics[taskName][msg] = time.Since(start)
	}
}

// get Metrics
func (pp *Profiler) Metrics() map[string]map[string]time.Duration {
	return pp.metrics
}

// debug logs for task metric
func (pp *Profiler) logProfileSummary(taskName string) {
	if durationMap, ok := pp.metrics[taskName]; ok {
		for k, v := range durationMap {
			if k != taskName {
				logger.V(4).Info("Subtask finished", "task_name", taskName, "subtask_name", k, "duration", v)
			}
		}
		if totalTaskDuration, ok := durationMap[taskName]; ok {
			logger.V(4).Info("Task finished", "task_name", taskName, "duration", totalTaskDuration)
			logger.V(4).Info("----------------------------------")
		}
	}
}

// Manages Task execution
type taskRunner struct {
	checkpointInfo CheckpointInfo
	writer         filewriter.FileWriter
	task           Task
}

// executes Task
func (pr *taskRunner) RunTask(ctx context.Context, commandContext *CommandContext) error {
	commandContext.Profiler = &Profiler{
		metrics: make(map[string]map[string]time.Duration),
		starts:  make(map[string]map[string]time.Time),
	}
	task := pr.task
	start := time.Now()
	defer taskRunnerFinalBlock(start)

	checkpointInfo := newCheckpointInfo(commandContext.ClusterSpec.Cluster.Name)
	checkpointFileName := filepath.Join(commandContext.Writer.Dir(), "generated", checkpointInfo.Filename)

	if _, err := os.Stat(checkpointFileName); err == nil {
		logger.V(4).Info("File already exists", "file", checkpointFileName)

		checkpointFile := GetCheckpointFile(checkpointFileName)
		if checkpointFile.CompletedTasks != nil {
			checkpointInfo.CompletedTasks = checkpointFile.CompletedTasks
		}
	}

	for task != nil {
		if _, ok := checkpointInfo.CompletedTasks[task.Name()]; ok {
			task = checkpointInfo.CompletedTasks[task.Name()].NextTask
			continue
		}
		logger.V(4).Info("Task start", "task_name", task.Name())
		commandContext.Profiler.SetStartTask(task.Name())
		nextTask := task.Run(ctx, commandContext)
		commandContext.Profiler.MarkDoneTask(task.Name())
		commandContext.Profiler.logProfileSummary(task.Name())
		if commandContext.OriginalError == nil {
			checkpointInfo.taskCompleted(task.Name(), task.Checkpoint(nextTask))
		}
		task = nextTask
	}
	if commandContext.OriginalError != nil {
		pr.saveCheckpoint(checkpointInfo)
	}
	return commandContext.OriginalError
}

func taskRunnerFinalBlock(startTime time.Time) {
	logger.V(4).Info("Tasks completed", "duration", time.Since(startTime))
}

func NewTaskRunner(task Task, writer filewriter.FileWriter) *taskRunner {
	return &taskRunner{
		task:   task,
		writer: writer,
	}
}

func (pr *taskRunner) saveCheckpoint(checkpointInfo CheckpointInfo) {
	log.Printf("Saving checkpoint:\n%v\n", checkpointInfo)
	content, err := yaml.Marshal(checkpointInfo)
	if err != nil {
		log.Printf("failed saving task runner checkpoint: %v\n", err)
	}

	if _, err = pr.writer.Write(checkpointInfo.Filename, content); err != nil {
		log.Printf("failed saving task runner checkpoint: %v\n", err)
	}
}

type TaskCheckpoint struct {
	NextTask Task
}

type CheckpointInfo struct {
	Filename       string                    `json:"filename"`
	CompletedTasks map[string]TaskCheckpoint `json:"completedTasks"`
}

func newCheckpointInfo(clusterName string) CheckpointInfo {
	return CheckpointInfo{
		Filename:       fmt.Sprintf("%s-checkpoint.yaml", clusterName),
		CompletedTasks: make(map[string]TaskCheckpoint),
	}
}

func (c CheckpointInfo) taskCompleted(name string, info TaskCheckpoint) {
	c.CompletedTasks[name] = info
}

func GetCheckpointFile(file string) CheckpointInfo {
	logger.Info("Reading checkpoint", "file", file)
	content, err := os.ReadFile(file)
	if err != nil {
		log.Printf("failed reading checkpoint file: %v\n", err)
	}
	checkpointInfo := &CheckpointInfo{}
	err = yaml.Unmarshal(content, checkpointInfo)
	if err != nil {
		log.Printf("failed unmarshalling checkpoint: %v\n", err)
	}

	return *checkpointInfo
}
