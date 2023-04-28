package internal

type BridgesConfig map[string]*NodeConfig
type SequencersConfig map[string]*NodeConfig

type Config struct {
	Bridges    BridgesConfig    `toml:"bridges" mapstructure:"bridges"`
	Sequencers SequencersConfig `toml:"sequencers" mapstructure:"sequencers"`
	LogLevel   string           `toml:"log_level" mapstructure:"log_level"`
	Metrics    MetricsConfig    `toml:"metrics"`
}

type NodeConfig struct {
	OpNodePublicRpcUrl string `toml:"op_node_public_rpc_url"  mapstructure:"op_node_public_rpc_url"`
}
type MetricsConfig struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}
