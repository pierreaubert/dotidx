package dix

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type MgrConfig struct {
	TargetDir             string                                `toml:"target_dir"`
	Name                  string                                `toml:"name"`
	UnixUser              string                                // Runtime: set from environment
	SystemMemoryGB        int                                   // Runtime: detected system memory in GB
	MaintenanceWorkMemory string                                // Runtime: calculated maintenance_work_mem
	MaxWalSize            string                                // Runtime: calculated max_wal_size
	DbCache               int                                   // Runtime: calculated db_cache
	RpcMaxConnections     int                                   // Runtime: calculated rpc_max_connections
	DotidxRoot            string                                `toml:"dotidx_root"`
	DotidxBackup          string                                `toml:"dotidx_backup"`
	DotidxRun             string                                `toml:"dotidx_run"`
	DotidxRuntime         string                                `toml:"dotidx_runtime"`
	DotidxLogs            string                                `toml:"dotidx_logs"`
	DotidxBin             string                                `toml:"dotidx_bin"`
	DotidxStatic          string                                `toml:"dotidx_static"`
	DotidxBatch           DotidxBatch                           `toml:"dotidx_batch"`
	DotidxDB              DotidxDB                              `toml:"dotidx_db"`
	DotidxFE              DotidxFE                              `toml:"dotidx_fe"`
	Parachains            map[string]map[string]ParaChainConfig `toml:"parachains"`
	Filesystem            FilesystemConfig                      `toml:"filesystem"`
	Monitoring            MonitoringConfig                      `toml:"monitoring"`
	Watcher               OrchestratorConfig                    `toml:"watcher"`
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

type OrchestratorConfig struct {
	WatchInterval    time.Duration `toml:"watch_interval"`
	MaxRestarts      int           `toml:"max_restarts"`
	RestartBackoff   time.Duration `toml:"restart_backoff"`
	OperationTimeout time.Duration `toml:"operation_timeout"`
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

// GetSystemMemoryGB detects the system's total memory in GB
func GetSystemMemoryGB() (int, error) {
	var memBytes uint64

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize")
		output, err := cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("failed to get memory on macOS: %w", err)
		}
		memBytes, err = strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse memory value: %w", err)
		}

	case "linux":
		cmd := exec.Command("grep", "MemTotal", "/proc/meminfo")
		output, err := cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("failed to get memory on Linux: %w", err)
		}
		// MemTotal:       16384000 kB
		fields := strings.Fields(string(output))
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected meminfo format")
		}
		memKB, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse memory value: %w", err)
		}
		memBytes = memKB * 1024

	default:
		return 16, nil // default fallback for unsupported platforms
	}

	// Convert bytes to GB (rounded)
	memGB := int((memBytes + (1024*1024*1024)/2) / (1024 * 1024 * 1024))
	return memGB, nil
}

// CalculateMemorySettings calculates PostgreSQL and Node memory settings based on system memory
func (c *MgrConfig) CalculateMemorySettings() {
	if c.SystemMemoryGB <= 0 {
		c.SystemMemoryGB = 16 // default fallback
	}

	// Calculate maintenance_work_mem: (systemMemory / 16) * 4GB, capped at 64GB
	maintenanceGB := (c.SystemMemoryGB * 4) / 16
	if maintenanceGB > 64 {
		maintenanceGB = 64
	}
	if maintenanceGB < 1 {
		maintenanceGB = 1
	}
	c.MaintenanceWorkMemory = fmt.Sprintf("%dGB", maintenanceGB)

	// Calculate max_wal_size: (systemMemory / 16) * 1GB, capped at 4GB
	walGB := c.SystemMemoryGB / 16
	if walGB > 4 {
		walGB = 4
	}
	if walGB < 1 {
		walGB = 1
	}
	c.MaxWalSize = fmt.Sprintf("%dGB", walGB)

	// Reference values for Node: 16GB RAM -> 1GB db-cache, 2k rpc-max-connections
	// Scales linearly with caps.
	const baseMemory = 16
	const maxMemory = 128 // Memory size in GB where max settings are reached

	if c.SystemMemoryGB <= baseMemory {
		c.DbCache = 1024
		c.RpcMaxConnections = 2000
	} else {
		// Linear scaling between baseMemory and maxMemory
		scalingFactor := float64(c.SystemMemoryGB-baseMemory) / float64(maxMemory-baseMemory)

		// DbCache (1GB to 16GB)
		dbCache := 1024 + scalingFactor*(16384-1024)
		if dbCache > 16384 {
			dbCache = 16384
		}
		c.DbCache = int(dbCache)

		// RpcMaxConnections (2k to 16k)
		rpcMaxConnections := 2000 + scalingFactor*(16000-2000)
		if rpcMaxConnections > 16000 {
			rpcMaxConnections = 16000
		}
		c.RpcMaxConnections = int(rpcMaxConnections)
	}
}
