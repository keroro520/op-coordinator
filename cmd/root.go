package cmd

import (
	"context"
	"fmt"
	"github.com/node-real/op-coordinator/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

const (
	Version = "0.0.1"
)

var rootCmd = &cobra.Command{
	Use:     "coordinator",
	Short:   "coordinator CLI",
	Version: Version,
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(
		StartCommand(),
	)
	rootCmd.AddCommand(
		VersionCommand(),
	)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.toml", "config file")
}

func StartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "print service version",
		RunE:  startHandleFunc,
	}
	return cmd
}

func startHandleFunc(cmd *cobra.Command, args []string) error {
	fmt.Printf("Start service, version is %v\n", Version)
	coordinator, err := internal.NewCoordinator(config)
	if err != nil {
		zap.S().Error("Fail to create coordinator, error: %+v", err)
		return err
	}

	// TODO context and signal
	coordinator.Start(context.Background())
	return nil
}

func VersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "print service version",
		RunE:  VersionHandleFunc,
	}
	return cmd
}
func VersionHandleFunc(cmd *cobra.Command, args []string) error {
	fmt.Printf("service version is %v\n", Version)
	return nil
}

var config internal.Config

func initConfig() {

	fmt.Printf("config file: %s\n", cfgFile)
	// Use config file from the flag.
	viper.SetConfigFile(cfgFile)
	viper.SetConfigType("toml")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
	err := viper.Unmarshal(&config)
	if err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
	fmt.Printf("config file: %#v\n", config)

	err = createLogger()
	if err != nil {
		panic("create log error")
	}
}

func createLogger() error {
	level, err := zapcore.ParseLevel(config.LogLevel)
	if err != nil {
		return err
	}
	productionConfig := zap.NewProductionConfig()
	productionConfig.Level.SetLevel(level)

	logger, err := productionConfig.Build()
	if err != nil {
		return err
	}

	_ = zap.ReplaceGlobals(logger)
	return nil
}

func Execute() error {
	return rootCmd.Execute()
}
