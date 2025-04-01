package dotidx

import (
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type MgrConfig struct {
	TargetDir     string                                `toml:"target_dir"`
	Name          string                                `toml:"name"`
	DotidxRoot    string                                `toml:"dotidx_root"`
	DotidxBackup  string                                `toml:"dotidx_backup"`
	DotidxRun     string                                `toml:"dotidx_run"`
	DotidxRuntime string                                `toml:"dotidx_runtime"`
	DotidxLogs    string                                `toml:"dotidx_logs"`
	DotidxBin     string                                `toml:"dotidx_bin"`
	DotidxStatic  string                                `toml:"dotidx_static"`
	DotidxBatch   DotidxBatch                           `toml:"dotidx_batch"`
	DotidxDB      DotidxDB                              `toml:"dotidx_db"`
	DotidxFE      DotidxFE                              `toml:"dotidx_fe"`
	Parachains    map[string]map[string]ParaChainConfig `toml:"parachains"`
	Filesystem    FilesystemConfig                      `toml:"filesystem"`
	Monitoring    MonitoringConfig                      `toml:"monitoring"`
}

type DotidxDB struct {
	Type          string   `toml:"type"`
	Version       int      `toml:"version"`
	Name          string   `toml:"name"`
	IP            string   `toml:"ip"`
	User          string   `toml:"user"`
	Port          int      `toml:"port"`
	Password      string   `toml:"password"`
	Memory        string   `toml:"memory"`
	Data          string   `toml:"data"`
	Run           string   `toml:"run"`
	WhitelistedIP []string `toml:"whitelisted_ip"`
}

type Duration time.Duration

type DotidxBatch struct {
	StartRange   int      `toml:"start_range"`
	EndRange     int      `toml:"end_range"`
	BatchSize    int      `toml:"batch_size"`
	MaxWorkers   int      `toml:"max_workers"`
	FlushTimeout Duration `toml:"flush_timeout"`
}

type DotidxFE struct {
	IP         string `toml:"ip"`
	Port       int    `toml:"port"`
	StaticPath string `toml:"static_path"`
}

type ParaChainConfig struct {
	Name            string `toml:"name"`
	Bin             string `toml:"bin"`
	PortRPC         int    `toml:"port_rpc"`
	PortWS          int    `toml:"port_ws"`
	Basepath        string `toml:"basepath"`
	ChainreaderIP   string `toml:"chainreader_ip"`
	ChainreaderPort int    `toml:"chainreader_port"`
	SidecarIP       string `toml:"sidecar_ip"`
	SidecarPort     int    `toml:"sidecar_port"`
	SidecarCount    int    `toml:"sidecar_count"`
	PrometheusPort  int    `toml:"prometheus_port"`
	RelayIP         string `toml:"relay_ip"`
	NodeIP          string `toml:"node_ip"`
}

func (ParaChainConfig) ComputePort(i, j int) int {
	return i + j + 1
}

type FilesystemConfig struct {
	ZFS bool `toml:"zfs"`
}

type MonitoringConfig struct {
	User           string `toml:"user"`
	PrometheusIP   string `toml:"prometheus_ip"`
	PrometheusPort int    `toml:"prometheus_port"`
	GrafanaIP      string `toml:"grafana_ip"`
	GrafanaPort    int    `toml:"grafana_port"`
}

func LoadMgrConfig(file string) (*MgrConfig, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config MgrConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func (d *Duration) UnmarshalText(b []byte) error {
	x, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(x)
	return nil
}

func DBUrl(config MgrConfig) string {
	return fmt.Sprintf(`%s://%s:%s@%s:%d/%s?sslmode=disable`,
		config.DotidxDB.Type,
		config.DotidxDB.User,
		config.DotidxDB.Password,
		config.DotidxDB.IP,
		config.DotidxDB.Port,
		config.DotidxDB.Name,
	)
}

func DBUrlSecure(config MgrConfig) string {
	return fmt.Sprintf(`%s://%s:******@%s:%d/%s?sslmode=disable`,
		config.DotidxDB.Type,
		config.DotidxDB.User,
		config.DotidxDB.IP,
		config.DotidxDB.Port,
		config.DotidxDB.Name,
	)
}
