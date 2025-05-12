package dix

import (
	"fmt"
	"log"
	"os"
	"sync"
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
	Watcher       OrchestratorSettings                  `toml:"watcher"`
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
	Name                  string `toml:"name"`
	Bin                   string `toml:"bin"`
	PortRPC               int    `toml:"port_rpc"`
	PortWS                int    `toml:"port_ws"`
	Basepath              string `toml:"basepath"`
	ChainreaderIP         string `toml:"chainreader_ip"`
	ChainreaderPort       int    `toml:"chainreader_port"`
	SidecarIP             string `toml:"sidecar_ip"`
	SidecarPort           int    `toml:"sidecar_port"`
	SidecarPrometheusPort int    `toml:"sidecar_prometheus_port"`
	SidecarCount          int    `toml:"sidecar_count"`
	PrometheusPort        int    `toml:"prometheus_port"`
	RelayIP               string `toml:"relay_ip"`
	NodeIP                string `toml:"node_ip"`
	BootNodes             string `toml:"bootnodes"`
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

type OrchestratorSettings struct {
	WatchIntervalSeconds    int `toml:"watch_interval_seconds"`
	MaxRestarts             int `toml:"max_restarts"`
	RestartBackoffSeconds   int `toml:"restart_backoff_seconds"`
	OperationTimeoutSeconds int `toml:"operation_timeout_seconds"`
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

func (mc *MgrConfig) GetServiceTree() ([]*ServiceNode, error) {
	var rootNodes []*ServiceNode

	parachainParentNode := &ServiceNode{
		Name:     "Parachains",
		IsLeaf:   false,
		Children: []*ServiceNode{},
	}

	for groupName, group := range mc.Parachains {
		for parachainKey, parachainCfg := range group {
			systemdUnitName := fmt.Sprintf("%s.service", parachainCfg.Name)
			if parachainCfg.Name == "" {
				log.Printf("Warning: Parachain in group '%s', key '%s' has no name, cannot form systemd unit name. Skipping.", groupName, parachainKey)
				continue
			}

			node := &ServiceNode{
				Name:        fmt.Sprintf("%s-%s", groupName, parachainCfg.Name),
				SystemdUnit: systemdUnitName,
				IsLeaf:      true,
				Children:    nil,
			}
			parachainParentNode.Children = append(parachainParentNode.Children, node)
		}
	}

	if len(parachainParentNode.Children) > 0 {
		rootNodes = append(rootNodes, parachainParentNode)
	}

	return rootNodes, nil
}

func (mc *MgrConfig) GetManagerConfig() (ManagerConfig, error) {
	cfg := ManagerConfig{
		WatchInterval:    30 * time.Second,
		MaxRestarts:      5,
		RestartBackoff:   10 * time.Second,
		OperationTimeout: 60 * time.Second,
	}

	if mc.Watcher.WatchIntervalSeconds > 0 {
		cfg.WatchInterval = time.Duration(mc.Watcher.WatchIntervalSeconds) * time.Second
	}
	if mc.Watcher.MaxRestarts > 0 {
		cfg.MaxRestarts = mc.Watcher.MaxRestarts
	}
	if mc.Watcher.RestartBackoffSeconds > 0 {
		cfg.RestartBackoff = time.Duration(mc.Watcher.RestartBackoffSeconds) * time.Second
	}
	if mc.Watcher.OperationTimeoutSeconds > 0 {
		cfg.OperationTimeout = time.Duration(mc.Watcher.OperationTimeoutSeconds) * time.Second
	}

	return cfg, nil
}

var globalMgrConfig *MgrConfig
var loadConfigOnce sync.Once
var loadConfigError error

func InitGlobalConfig(configFile string) error {
	loadConfigOnce.Do(func() {
		globalMgrConfig, loadConfigError = LoadMgrConfig(configFile)
	})
	if loadConfigError != nil {
		return fmt.Errorf("failed to load global manager configuration from %s: %w", configFile, loadConfigError)
	}
	if globalMgrConfig == nil && loadConfigError == nil {
		return fmt.Errorf("global manager configuration is nil after loading from %s, and no error was reported", configFile)
	}
	return loadConfigError
}

func GetServiceTree() ([]*ServiceNode, error) {
	if globalMgrConfig == nil {
		return nil, fmt.Errorf("manager configuration not loaded; call InitGlobalConfig first")
	}
	return globalMgrConfig.GetServiceTree()
}

func GetManagerConfig() (ManagerConfig, error) {
	if globalMgrConfig == nil {
		return ManagerConfig{}, fmt.Errorf("manager configuration not loaded; call InitGlobalConfig first")
	}
	return globalMgrConfig.GetManagerConfig()
}
