package cmd

import (
	"context"
	"fmt"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/log"
	config_ "github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/core"
	"github.com/node-real/op-coordinator/metrics"
	"github.com/node-real/op-coordinator/rpc"
	"github.com/node-real/op-coordinator/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"time"
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

	// Create clients for nodes
	var err error
	nodes := make(map[string]*types.Node)
	for nodeName, nodeCfg := range config.Candidates {
		nodes[nodeName], err = types.NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return err
		}
	}
	for nodeName, nodeCfg := range config.Bridges {
		nodes[nodeName], err = types.NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return err
		}
	}

	// Create Health Checker
	hc := core.NewHealthChecker(
		time.Duration(config.HealthCheck.IntervalMs)*time.Millisecond,
		config.HealthCheck.FailureThresholdLast5,
		logger,
	)
	go hc.Start(context.Background(), &nodes)

	c := core.NewCoordinator(config, hc, nodes, logger)

	server := rpc.NewRPCServer(config, "v1.0", c, logger)
	err = server.Start()
	if err != nil {
		logger.Error("Start rpc server", "error", err)
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
var logger log.Logger

func initConfig() {
	fmt.Printf("config file: %s\n", cfgFile)
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

	logger = createLogger(config.LogLevel)
}

func createLogger(logLevel string) log.Logger {
	logCfg := oplog.DefaultCLIConfig()
	logCfg.Level = logLevel
	return oplog.NewLogger(logCfg)
}

func Execute() error {
	return rootCmd.Execute()
}
