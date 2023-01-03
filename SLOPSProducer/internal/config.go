package internal

import "gopkg.in/yaml.v2"

type Config struct {
	Service            string `yaml:"service"`
	UpdateInterval     int    `yaml:"update_interval"`
	UpdateIntervalUnit string `yaml:"update_interval_unit"`
	Threshold          int    `yaml:"threshold"`
	ChgPercent         int    `yaml:"chg_percent"`
	Partitions         int    `yaml:"partitions"`
	GRPCPort           int    `yaml:"grpc_port"`
	HTTPPort           int    `yaml:"http_port"`
}

func (c *Config) Parse(data []byte) error {
	return yaml.Unmarshal(data, c)
}
