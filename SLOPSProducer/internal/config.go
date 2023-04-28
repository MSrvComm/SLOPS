package internal

import "gopkg.in/yaml.v2"

type Config struct {
	Service            string  `yaml:"service"`
	UpdateInterval     int     `yaml:"update_interval"`
	UpdateIntervalUnit string  `yaml:"update_interval_unit"`
	FreqThreshold      int32   `yaml:"freq_threshold"`
	SampleThreshold    float64 `yaml:"sample_threshold"`
	ChgPercent         int     `yaml:"chg_percent"`
	Support            float64 `yaml:"support"`
	Epsilon            float64 `yaml:"epsilon"`
	Partitions         int32   `yaml:"partitions"`
	GRPCPort           int     `yaml:"grpc_port"`
	HTTPPort           int     `yaml:"http_port"`
	SwapInterval       int     `yaml:"swap_interval"`
}

func (c *Config) Parse(data []byte) error {
	return yaml.Unmarshal(data, c)
}
