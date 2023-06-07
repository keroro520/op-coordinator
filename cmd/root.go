package cmd

import (
	"context"
	"fmt"
	"github.com/node-real/op-coordinator/internal"
	"github.com/node-real/op-coordinator/internal/bridge"
	config_ "github.com/node-real/op-coordinator/internal/config"
	"github.com/node-real/op-coordinator/internal/coordinator"
	"github.com/node-real/op-coordinator/internal/metrics"
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

// TODO context and exit signal
func startHandleFunc(cmd *cobra.Command, args []string) error {
	fmt.Printf("Start service, version is %v\n", Version)

	c, err := coordinator.NewCoordinator(config)
	if err != nil {
		zap.S().Error("Fail to create coordinator, error: %+v", err)
		return err
	}

	h, err := bridge.NewHighestBridge(config)
	if err != nil {
		zap.S().Error("Fail to create highest bridge, error: %+v", err)
		return err
	}
	go h.Start(context.Background())

	server := internal.NewRPCServer(config, "v1.0", c, h)
	err = server.Start()
	if err != nil {
		zap.S().Errorf("Fail to start rpc server, error: %v", err)
		return err
	}

	metricsAddr := fmt.Sprintf("%s:%d", config.Metrics.Host, config.Metrics.Port)
	metrics.StartMetrics(metricsAddr)

	c.Start(context.Background())

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

var config config_.Config

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
	if err = config.Check(); err != nil {
		panic(fmt.Errorf("invalid config: %s", err))
	}

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
