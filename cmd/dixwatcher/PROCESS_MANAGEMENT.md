# DixWatcher Process Management

## Overview

DixWatcher now includes a pluggable process management system that allows you to manage processes either through **systemd** or **directly** without systemd. This provides flexibility for different deployment scenarios and removes the hard dependency on systemd.

## Architecture

### ProcessManager Interface

The `ProcessManager` interface provides a unified API for process lifecycle management:

```go
type ProcessManager interface {
    Start(ctx context.Context, config ProcessConfig) error
    Stop(ctx context.Context, name string) error
    Restart(ctx context.Context, name string) error
    GetStatus(ctx context.Context, name string) (*ProcessStatus, error)
    GetOutput(ctx context.Context, name string, lines int) ([]string, error)
    Kill(ctx context.Context, name string) error
    List(ctx context.Context) ([]string, error)
    Close() error
    Name() string
}
```

### Available Backends

1. **SystemdManager** - Manages processes via systemd (D-Bus)
2. **DirectManager** - Manages processes directly without systemd

## Configuration

### Command-Line Flags

```bash
# Process manager type
-process-manager string
    Process manager type: systemd or direct (default "systemd")

# Direct mode configuration
-process-log-dir string
    Directory for process logs (direct mode) (default "/var/log/dixwatcher")
-process-pid-dir string
    Directory for PID files (direct mode) (default "/var/run/dixwatcher")
-process-max-restarts int
    Maximum restart attempts per process (default 5)
```

### Usage Examples

#### Using Systemd (Default)

```bash
./dixwatcher -conf config.toml -exec -process-manager systemd
```

#### Using Direct Process Management

```bash
./dixwatcher -conf config.toml -exec \
  -process-manager direct \
  -process-log-dir /var/log/myapp \
  -process-pid-dir /var/run/myapp \
  -process-max-restarts 10
```

## Process Configuration

### ProcessConfig Structure

```go
type ProcessConfig struct {
    // Basic configuration
    Name        string   // Unique process name
    Command     string   // Command to execute
    Args        []string // Command arguments
    WorkingDir  string   // Working directory
    Environment []string // Environment variables (KEY=VALUE format)

    // User/Group (requires root or capabilities)
    User  string // Username or UID
    Group string // Group name or GID

    // Resource limits
    CPULimit    float64 // CPU limit in cores (0 = unlimited)
    MemoryLimit int64   // Memory limit in bytes (0 = unlimited)
    IOWeight    int     // I/O weight (0-1000, 0 = default)

    // Restart policy
    RestartPolicy RestartPolicy // never, on-failure, always
    RestartDelay  time.Duration // Delay before restart

    // Output handling
    CaptureOutput bool   // Whether to capture stdout/stderr
    LogFile       string // Optional log file path

    // Health checking
    HealthCheck *HealthCheckConfig
}
```

### Restart Policies

- **`RestartNever`** - Never restart the process
- **`RestartOnFailure`** - Restart only if the process exits with non-zero code
- **`RestartAlways`** - Always restart the process when it stops

## DirectManager Features

### Process Lifecycle

1. **Spawn Process**
   - Creates exec.Cmd with configured command and arguments
   - Sets working directory and environment
   - Sets user/group (if running as root)
   - Creates PID file

2. **Monitor Process**
   - Tracks process state (starting, running, stopping, stopped, failed)
   - Monitors exit code and errors
   - Handles automatic restart based on policy
   - Records metrics and health events

3. **Graceful Shutdown**
   - Sends SIGTERM signal
   - Waits up to 10 seconds for graceful shutdown
   - Escalates to SIGKILL if necessary
   - Cleans up PID files and log files

### Output Capture

DirectManager uses a **ring buffer** to efficiently capture and store process output:

- Stores last 1000 lines per process
- Captures both stdout and stderr
- Optionally writes to log files
- Provides GetOutput API to retrieve recent lines

### Example: Starting a Process

```go
config := ProcessConfig{
    Name:          "myapp",
    Command:       "/usr/bin/node",
    Args:          []string{"server.js"},
    WorkingDir:    "/opt/myapp",
    Environment:   []string{"NODE_ENV=production", "PORT=3000"},
    RestartPolicy: RestartAlways,
    RestartDelay:  5 * time.Second,
    CaptureOutput: true,
    LogFile:       "/var/log/myapp/output.log",
}

err := processManager.Start(ctx, config)
```

## Temporal Activities

The process management system provides the following Temporal activities:

### StartProcessActivity

Starts a process using the configured process manager.

```go
input := ProcessConfig{
    Name:    "blockchain-node",
    Command: "/usr/bin/polkadot",
    Args:    []string{"--chain", "kusama"},
    // ... other config
}
err := workflow.ExecuteActivity(ctx, activities.StartProcessActivity, input).Get(ctx, nil)
```

### StopProcessActivity

Gracefully stops a running process.

```go
err := workflow.ExecuteActivity(ctx, activities.StopProcessActivity, "blockchain-node").Get(ctx, nil)
```

### RestartProcessActivity

Restarts a process (stop + start).

```go
err := workflow.ExecuteActivity(ctx, activities.RestartProcessActivity, "blockchain-node").Get(ctx, nil)
```

### CheckProcessActivity

Checks the status and health of a process.

```go
var status ProcessStatus
err := workflow.ExecuteActivity(ctx, activities.CheckProcessActivity, "blockchain-node").Get(ctx, &status)
// status contains: State, PID, RestartCount, CPUPercent, MemoryBytes, Healthy, etc.
```

### GetProcessOutputActivity

Retrieves recent output lines from a process.

```go
var output []string
err := workflow.ExecuteActivity(ctx, activities.GetProcessOutputActivity, "blockchain-node", 50).Get(ctx, &output)
// Returns last 50 lines of output
```

### KillProcessActivity

Forcefully kills a process (SIGKILL).

```go
err := workflow.ExecuteActivity(ctx, activities.KillProcessActivity, "blockchain-node").Get(ctx, nil)
```

### ListProcessesActivity

Lists all managed processes.

```go
var processes []string
err := workflow.ExecuteActivity(ctx, activities.ListProcessesActivity).Get(ctx, &processes)
```

## Integration with Other Features

### Metrics

Process management integrates with Prometheus metrics:

- `dixwatcher_activity_executions_total{activity="StartProcess"}`
- `dixwatcher_activity_executions_total{activity="StopProcess"}`
- `dixwatcher_activity_executions_total{activity="RestartProcess"}`
- `dixwatcher_activity_duration_seconds{activity="StartProcess"}`
- `dixwatcher_service_restarts_total{service="myapp", type="direct"}`

### Circuit Breakers

Process operations are protected by circuit breakers to prevent cascading failures:

- Automatic circuit opening after 5 consecutive failures
- 60-second timeout before attempting recovery
- Half-open state for testing recovery

### Health History

All process events are recorded in the health history database:

- Process starts and stops
- Restart events with reasons
- Health check results
- Resource usage metrics

### Alerting

Process failures trigger alerts through configured channels:

- Service failure alerts (severity: critical)
- Excessive restart alerts (severity: warning)
- Resource threshold alerts (CPU, memory)

## Migration Guide

### From Systemd-Only to Hybrid

If you're currently using systemd and want to migrate some services to direct management:

1. **Keep infrastructure services in systemd** (nginx, postgres, redis)
2. **Move application services to direct management** (your apps, blockchain nodes)

Example workflow:

```go
// Infrastructure service (systemd)
workflow.ExecuteActivity(ctx, activities.StartSystemdServiceActivity, "nginx")

// Application service (direct)
appConfig := ProcessConfig{
    Name:          "dixfe",
    Command:       "/opt/dixfe/bin/dixfe",
    RestartPolicy: RestartAlways,
    CaptureOutput: true,
}
workflow.ExecuteActivity(ctx, activities.StartProcessActivity, appConfig)
```

### From Direct to Systemd

To switch back to systemd, simply:

1. Change `-process-manager` flag to `systemd`
2. Ensure services are defined as systemd units
3. Restart dixwatcher

## Security Considerations

### User/Group Switching

When running as root, DirectManager can start processes as different users:

```go
config := ProcessConfig{
    Name:    "webapp",
    Command: "/usr/bin/node",
    User:    "www-data",
    Group:   "www-data",
    // ...
}
```

**Note:** Requires CAP_SETUID and CAP_SETGID capabilities or root.

### PID File Security

- PID files are created in `-process-pid-dir` (default: `/var/run/dixwatcher`)
- Ensure this directory has appropriate permissions (0755)
- PID files are automatically cleaned up on process exit

### Log File Security

- Log files are created in `-process-log-dir` (default: `/var/log/dixwatcher`)
- Files are created with 0644 permissions
- Ensure log rotation is configured to prevent disk space issues

## Troubleshooting

### Process Won't Start

1. Check logs: `journalctl -u dixwatcher` or check log files
2. Verify command and arguments are correct
3. Check working directory exists and is accessible
4. Verify user/group permissions if using user switching
5. Check resource limits (ulimit, cgroups)

### Process Keeps Restarting

1. Check process exit code: `GetProcessOutputActivity` to see logs
2. Review restart policy configuration
3. Check `-process-max-restarts` limit
4. Review application logs for crash reasons
5. Check resource usage (CPU, memory limits)

### Output Not Captured

1. Ensure `CaptureOutput: true` in ProcessConfig
2. Check if process writes to stdout/stderr (some apps use files)
3. Verify log file path is writable
4. Check ring buffer size (fixed at 1000 lines)

### Permission Errors

1. Ensure dixwatcher has permissions to create PID/log directories
2. Check user/group switching requirements (need root)
3. Verify command path is executable
4. Check SELinux/AppArmor policies if applicable

## Performance Characteristics

### DirectManager

- **Memory**: ~10MB per managed process (includes ring buffer)
- **CPU**: Minimal overhead (<1% per process)
- **Startup Time**: 50-100ms per process
- **Graceful Shutdown**: Up to 10 seconds per process

### SystemdManager

- **Memory**: ~2MB per managed service (D-Bus overhead)
- **CPU**: Minimal overhead (~0.1% per service)
- **Startup Time**: 100-200ms per service (D-Bus calls)
- **Graceful Shutdown**: Depends on systemd timeout configuration

## Best Practices

1. **Use systemd for system services** - nginx, postgres, system daemons
2. **Use direct for application services** - your applications, blockchain nodes
3. **Set appropriate restart policies** - Use `RestartOnFailure` for most services
4. **Configure restart delays** - Prevent rapid restart loops (5-30 seconds)
5. **Limit max restarts** - Prevent infinite restart loops (5-10 attempts)
6. **Enable output capture** - Essential for debugging
7. **Use log files for long-running processes** - Ring buffer is limited
8. **Monitor metrics** - Track restarts, failures, resource usage
9. **Set resource limits** - Prevent resource exhaustion
10. **Test graceful shutdown** - Ensure processes handle SIGTERM properly

## Future Enhancements

Planned improvements:

- **Cgroups integration** - Enforce CPU/memory limits
- **Container support** - Docker/Podman backend
- **Kubernetes support** - Kubernetes Job/Deployment backend
- **Process groups** - Manage related processes together
- **Advanced health checks** - HTTP, TCP, exec-based checks
- **Dependency management** - Start/stop processes in order
- **Rolling restarts** - Restart without downtime
- **Blue/green deployments** - Seamless updates

## References

- [Temporal Documentation](https://docs.temporal.io/)
- [systemd Documentation](https://www.freedesktop.org/software/systemd/man/)
- [Linux Process Management](https://man7.org/linux/man-pages/man7/signal.7.html)
- [Go exec Package](https://pkg.go.dev/os/exec)
