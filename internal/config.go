package internal

type BridgesConfig map[string]*NodeConfig
type CandidatesConfig map[string]*NodeConfig

type Config struct {
	Bridges              BridgesConfig    `toml:"bridges" mapstructure:"bridges"`
	Candidates           CandidatesConfig `toml:"candidates" mapstructure:"candidates"`
	LogLevel             string           `toml:"log_level" mapstructure:"log_level"`
	Metrics              MetricsConfig    `toml:"metrics"`
	RPC                  RpcConfig        `toml:"rpc"`
	SleepTime            uint             `toml:"sleep_time"`
	HealthCheckWindow    uint             `toml:"health_check_window"`
	HealthCheckThreshold uint             `toml:"health_check_threshold"`
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
