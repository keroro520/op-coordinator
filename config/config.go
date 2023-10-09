package config

import (
	"errors"
)

type BridgesConfig map[string]*NodeConfig
type SequencersConfig map[string]*NodeConfig

type Config struct {
	Sequencers SequencersConfig `toml:"sequencers" mapstructure:"sequencers"`
	Bridges    BridgesConfig    `toml:"bridges" mapstructure:"bridges"`
	LogLevel   string           `toml:"log_level" mapstructure:"log_level"`
	Metrics    MetricsConfig    `toml:"metrics"`
	RPC        RpcConfig        `toml:"rpc"`
	SleepTime  uint             `toml:"sleep_time"`

	HealthCheck HealthCheckConfig `toml:"health_check" mapstructure:"health_check"`
	Election    ElectionConfig    `toml:"election" mapstructure:"election"`
	Forward     ForwardConfig     `toml:"forward" mapstructure:"forward"`
}

type NodeConfig struct {
	OpNodePublicRpcUrl string `toml:"op_node_public_rpc_url"  mapstructure:"op_node_public_rpc_url"`
	OpGethPublicRpcUrl string `toml:"op_geth_public_rpc_url"  mapstructure:"op_geth_public_rpc_url"`
}

type MetricsConfig struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}
type RpcConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type HealthCheckConfig struct {
	IntervalMs int64 `toml:"interval_ms" mapstructure:"interval_ms"`
}

type ElectionConfig struct {
	Stopped                        bool  `toml:"stopped" mapstructure:"stopped"`
	MaxWaitingTimeForConvergenceMs int64 `toml:"max_waiting_time_for_convergence_ms" mapstructure:"max_waiting_time_for_convergence_ms"`
	MinRequiredHealthyNodes        int   `toml:"min_required_healthy_nodes" mapstructure:"min_required_healthy_nodes"`
}

type ForwardConfig struct {
	SubSyncStatusUnsafeL2Number int `toml:"sub_sync_status_unsafe_l2_number" mapstructure:"sub_sync_status_unsafe_l2_number"`
}

func (cfg *Config) Check() error {
	// Check ElectionConfig
	if cfg.Election.MinRequiredHealthyNodes <= 0 {
		return errors.New("min_required_healthy_nodes must be greater than 0")
	}
	if cfg.Election.MinRequiredHealthyNodes > len(cfg.Sequencers)+len(cfg.Bridges) {
		return errors.New("min_required_healthy_nodes must be less than or equal to the number of sequencers and bridges")
	}

	return nil
}
