package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/aws/eks-anywhere/pkg/executables"
	"github.com/aws/eks-anywhere/pkg/filewriter"
	"github.com/aws/eks-anywhere/pkg/logger"
)

type nodeLogsOptions struct {
	privateKeyPath string
	username       string
	nodeIps        []string
}

const (
	logOutputNameFormat = "%s-%s.log"
)

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

	_ = nodeLogsCmd.MarkFlagRequired("privatekey")
}

func nodeLogs(ctx context.Context) error {
	writer, _ := filewriter.NewWriter("nodelogs")
	ssh := executables.NewLocalExecutablesBuilder().BuildSSHExecutable()
	for _, nodeIP := range nodeLogsOpt.nodeIps {
		stdout, err := ssh.RunCommand(ctx, nodeLogsOpt.privateKeyPath, nodeLogsOpt.username, nodeIP, "sudo sheltie apiclient get settings.network.hostname")
		if err != nil {
			return err
		}
		if err := writeNodeLogsToFile(writer, stdout, nodeIP, "logName"); err != nil {
			return fmt.Errorf("writing logs to file: %v", err)
		}
	}

	return nil
}

func writeNodeLogsToFile(writer filewriter.FileWriter, out bytes.Buffer, nodeIP, logName string) error {
	filename := fmt.Sprintf(logOutputNameFormat, logName, nodeIP)
	path, err := writer.Write(filename, out.Bytes())
	if err != nil {
		return err
	}
	logger.Info("Successfully fetched node logs", "path", path)
	return nil
}
