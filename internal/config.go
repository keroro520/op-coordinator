package internal

type BridgesConfig map[string]*NodeConfig
type CandidatesConfig map[string]*NodeConfig

type Config struct {
	Candidates CandidatesConfig `toml:"nodes" mapstructure:"nodes"`
	Bridges    BridgesConfig    `toml:"bridges" mapstructure:"bridges"`
	LogLevel   string           `toml:"log_level" mapstructure:"log_level"`
	Metrics    MetricsConfig    `toml:"metrics"`
	RPC        RpcConfig        `toml:"rpc"`
	SleepTime  uint             `toml:"sleep_time"`

	HealthCheck HealthCheckConfig `toml:"health_check" mapstructure:"health_check"`
	Election    ElectionConfig    `toml:"election mapstructure:"election"`
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
	IntervalMs            int64 `toml:"interval_ms"`
	FailureThresholdLast5 int   `toml:"failure_threshold_last5"`
}

type ElectionConfig struct {
	MaxWaitingTimeForConvergenceMs int64 `toml:"max_waiting_time_for_convergence_ms"`
}
