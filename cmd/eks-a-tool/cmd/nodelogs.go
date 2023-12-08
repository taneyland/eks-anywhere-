package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/aws/eks-anywhere/pkg/executables"
	"github.com/aws/eks-anywhere/pkg/logger"
)

type nodeLogsOptions struct {
	privateKeyPath string
	username       string
	nodeIps        []string
}

var nodeLogsCmd = &cobra.Command{
	Use:   "nodelogs",
	Short: "Get logs from cluster nodes",
	Long:  "Get container logs from cluster nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := nodeLogs(cmd.Context())
		if err != nil {
			log.Fatalf("Error getting container logs from cluster nodes: %v", err)
		}
		return nil
	},
}

var nodeLogsOpt = &nodeLogsOptions{}

func init() {
	rootCmd.AddCommand(nodeLogsCmd)
	nodeLogsCmd.Flags().StringVarP(&nodeLogsOpt.privateKeyPath, "privatekey", "k", "", "Private key file path")
	nodeLogsCmd.Flags().StringVarP(&nodeLogsOpt.username, "username", "u", "", "Username to access the nodes")
	nodeLogsCmd.Flags().StringSliceVarP(&nodeLogsOpt.nodeIps, "nodeip", "i", make([]string, 0), "IP addresses of the nodes")

	nodeLogsCmd.MarkFlagRequired("privatekey")
}

func nodeLogs(ctx context.Context) error {
	for _, nodeIP := range nodeLogsOpt.nodeIps {
		ssh := executables.NewLocalExecutablesBuilder().BuildSSHExecutable()
		stdout, err := ssh.RunCommand(ctx, nodeLogsOpt.privateKeyPath, nodeLogsOpt.username, nodeIP, "sudo sheltie apiclient get settings.network.hostname")
		if err != nil {
			return err
		}

		logger.Info(stdout)
	}

	return nil
}
