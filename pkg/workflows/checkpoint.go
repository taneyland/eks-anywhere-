package workflows

import (
	"fmt"
	"github.com/aws/eks-anywhere/pkg/logger"
	"log"
	"os"
	"sigs.k8s.io/yaml"
)

type TaskCheckpoint interface{}

type UnmarshallTaskCheckpoint func(config interface{}) error

type CheckpointInfo struct {
	Filename       string
	CompletedTasks map[string]bool `json:"completedTasks"`
}

func newCheckpointInfo(clusterName string) CheckpointInfo {
	return CheckpointInfo{
		Filename:       fmt.Sprintf("%s-checkpoint.yaml", clusterName),
		CompletedTasks: map[string]bool{},
	}
}

func (c CheckpointInfo) taskCompleted(name string) {
	c.CompletedTasks[name] = true
}

func GetCheckpointFile(file string) *CheckpointInfo {
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

	return checkpointInfo
}
