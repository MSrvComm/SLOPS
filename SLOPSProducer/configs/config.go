package configs

import "gopkg.in/yaml.v2"

type Config struct {
	Service         string  `yaml:"service"`
	SampleThreshold float64 `yaml:"sample_threshold"`
	Support         float64 `yaml:"support"`
	Epsilon         float64 `yaml:"epsilon"`
	Partitions      int32   `yaml:"partitions"`
	HTTPPort        int     `yaml:"http_port"`
	SwapInterval    int     `yaml:"swap_interval"`
}

func (c *Config) Parse(data []byte) error {
	return yaml.Unmarshal(data, c)
}
