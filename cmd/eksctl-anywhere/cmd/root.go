package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/eks-anywhere/pkg/logger"
)

var rootCmd = &cobra.Command{
	Use:              "anywhere",
	Short:            "Amazon EKS Anywhere",
	Long:             `Use eksctl anywhere to build your own self-managing cluster on your hardware with the best of Amazon EKS`,
	PersistentPreRun: rootPersistentPreRun,
}

func init() {
	rootCmd.PersistentFlags().IntP("verbosity", "v", 0, "Set the log level verbosity")
	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind flags for root: %v", err)
	}
	setWriters(rootCmd)
}

func setWriters(cmd *cobra.Command) {
	logFile, err := os.OpenFile("log.txt", os.O_CREATE | os.O_APPEND | os.O_RDWR, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	multi := io.MultiWriter(logFile, os.Stdout)
	cmd.SetOut(multi)
}

func rootPersistentPreRun(cmd *cobra.Command, args []string) {
	if err := initLogger(); err != nil {
		log.Fatal(err)
	}
}

func initLogger() error {
	if err := logger.InitZap(viper.GetInt("verbosity")); err != nil {
		return fmt.Errorf("failed init zap logger in root command: %v", err)
	}

	return nil
}

func Execute() error {
	return rootCmd.ExecuteContext(context.Background())
}
