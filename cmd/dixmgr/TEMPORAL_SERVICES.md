# Temporal Services for Batch and Cron Operations

This document describes the Temporal-based implementations of batch block indexing and cron query execution, which replace the standalone `dixbatch` and `dixcron` binaries.

## Overview

The new Temporal services provide:

1. **BatchWorkflow** - Replaces `dixbatch` for block indexing
2. **CronWorkflow** - Replaces `dixcron` for periodic query execution

### Benefits of Temporal Implementation

- **Better Observability**: View workflow status, history, and metrics in Temporal Web UI
- **Automatic Retries**: Built-in retry policies for transient failures
- **State Management**: Workflow state persists across restarts
- **Scalability**: Easy horizontal scaling by adding more workers
- **Continue-As-New**: Handles very long-running batch operations without history limits
- **Scheduling**: Native cron support eliminates custom ticker logic

## BatchWorkflow

### Purpose
Fetches blocks from a Polkadot parachain via Sidecar API and stores them in PostgreSQL.

### Configuration

```go
type BatchWorkflowConfig struct {
    RelayChain   string        // e.g., "polkadot"
    Chain        string        // e.g., "assethub"
    StartRange   int           // Starting block number
    EndRange     int           // Ending block (-1 = use chain head)
    BatchSize    int           // Blocks per batch request
    MaxWorkers   int           // Maximum parallel workers
    FlushTimeout time.Duration // Database flush timeout
    SidecarURL   string        // e.g., "http://localhost:8080"
}
```

### Starting a Batch Workflow

#### Using Temporal CLI

```bash
# Index blocks from 1 to current head
temporal workflow start \
  --task-queue dotidx-watcher \
  --type BatchWorkflow \
  --workflow-id wf.batch.polkadot.assethub \
  --input '{
    "RelayChain": "polkadot",
    "Chain": "assethub",
    "StartRange": 1,
    "EndRange": -1,
    "BatchSize": 100,
    "MaxWorkers": 10,
    "FlushTimeout": "30s",
    "SidecarURL": "http://localhost:8080"
  }'

# Index specific block range
temporal workflow start \
  --task-queue dotidx-watcher \
  --type BatchWorkflow \
  --workflow-id wf.batch.polkadot.assethub.1M-2M \
  --input '{
    "RelayChain": "polkadot",
    "Chain": "assethub",
    "StartRange": 1000000,
    "EndRange": 2000000,
    "BatchSize": 100,
    "MaxWorkers": 20,
    "FlushTimeout": "30s",
    "SidecarURL": "http://localhost:8080"
  }'
```

#### Using Go Client

```go
package main

import (
    "context"
    "time"

    "go.temporal.io/sdk/client"
)

func startBatchIndexing() error {
    c, err := client.Dial(client.Options{
        HostPort:  "localhost:7233",
        Namespace: "dotidx",
    })
    if err != nil {
        return err
    }
    defer c.Close()

    config := BatchWorkflowConfig{
        RelayChain:   "polkadot",
        Chain:        "assethub",
        StartRange:   1,
        EndRange:     -1, // Use current head
        BatchSize:    100,
        MaxWorkers:   10,
        FlushTimeout: 30 * time.Second,
        SidecarURL:   "http://localhost:8080",
    }

    workflowOptions := client.StartWorkflowOptions{
        ID:        "wf.batch.polkadot.assethub",
        TaskQueue: "dotidx-watcher",
    }

    we, err := c.ExecuteWorkflow(context.Background(), workflowOptions,
        "BatchWorkflow", config)
    if err != nil {
        return err
    }

    log.Printf("Started batch workflow: %s", we.GetID())
    return nil
}
```

### Workflow Behavior

1. **Get Chain Head**: If `EndRange` is -1, fetches current chain head
2. **Process in Chunks**: Processes blocks in 100k chunks to avoid large ranges
3. **Check Existing Blocks**: Queries database to skip already-indexed blocks
4. **Group into Batches**: Groups consecutive missing blocks for efficient batch processing
5. **Parallel Processing**: Processes batches concurrently up to `MaxWorkers`
6. **Continue-As-New**: After 500k blocks, uses Continue-As-New to reset workflow history

### Activities Used

- `GetChainHeadActivity` - Get current blockchain head
- `CheckExistingBlocksActivity` - Check which blocks exist in DB
- `ProcessSingleBlockActivity` - Fetch and store single block
- `ProcessBlockBatchActivity` - Fetch and store batch of blocks

## CronWorkflow

### Purpose
Executes registered SQL queries periodically to compute statistics and aggregations.

### Configuration

```go
type CronWorkflowConfig struct {
    HourlyCronSchedule string   // e.g., "0 * * * *"
    DailyCronSchedule  string   // e.g., "0 0 * * *"
    RegisteredQueries  []string // e.g., ["total_blocks_in_month", "total_addresses_in_month"]
}
```

### Starting a Cron Workflow

#### Hourly Execution (Current Month Blocks)

```bash
temporal workflow start \
  --task-queue dotidx-watcher \
  --type CronWorkflow \
  --workflow-id wf.cron.hourly \
  --cron "0 * * * *" \
  --input '{
    "HourlyCronSchedule": "0 * * * *",
    "DailyCronSchedule": "0 0 * * *",
    "RegisteredQueries": ["total_blocks_in_month"]
  }'
```

#### Daily Execution (All Historical Queries)

```bash
temporal workflow start \
  --task-queue dotidx-watcher \
  --type CronWorkflow \
  --workflow-id wf.cron.daily \
  --cron "0 0 * * *" \
  --input '{
    "HourlyCronSchedule": "0 * * * *",
    "DailyCronSchedule": "0 0 * * *",
    "RegisteredQueries": ["total_blocks_in_month", "total_addresses_in_month"]
  }'
```

### Workflow Behavior

#### Hourly Execution
- Executes only `total_blocks_in_month` for **current month**
- Updates block count continuously as new blocks are indexed

#### Daily Execution
- Executes **all registered queries** for **all past months** (2019 onwards)
- Skips months that already have results
- Processes all indexed chains automatically

### Registered Queries

#### 1. total_blocks_in_month
Counts total blocks indexed in a given month.

```sql
SELECT COUNT(*) as total_blocks
FROM chain.{{.Relaychain}}_{{.Chain}}_blocks
WHERE created_at >= '{{.Year}}-{{.Month}}-01'
  AND created_at < '{{.Year}}-{{.Month}}-01'::date + INTERVAL '1 month'
```

#### 2. total_addresses_in_month
Counts unique addresses active in a given month.

```sql
WITH month_bounds AS (
    SELECT
        MIN(block_id) as min_block,
        MAX(block_id) as max_block
    FROM chain.{{.Relaychain}}_{{.Chain}}_blocks
    WHERE created_at >= '{{.Year}}-{{.Month}}-01'
      AND created_at < '{{.Year}}-{{.Month}}-01'::date + INTERVAL '1 month'
)
SELECT COUNT(DISTINCT address) as total_addresses
FROM chain.{{.Relaychain}}_{{.Chain}}_address_to_blocks atb
CROSS JOIN month_bounds mb
WHERE atb.block_id >= mb.min_block
  AND atb.block_id <= mb.max_block
```

### Activities Used

- `GetDatabaseInfoActivity` - Get all indexed chains
- `CheckQueryResultExistsActivity` - Check if query result exists
- `ExecuteAndStoreNamedQueryActivity` - Execute and store query result
- `RegisterDefaultQueriesActivity` - Register default queries

## Database Integration

The Temporal services require a database connection to be set in the Activities:

```go
// In main.go, after creating activities:
if config.Database != nil {
    dbAdapter := NewDixDatabaseAdapter(database)
    activities.SetDatabase(dbAdapter)

    // Register default queries
    ctx := context.Background()
    activities.RegisterDefaultQueriesActivity(ctx)
}
```

## Monitoring and Observability

### Temporal Web UI
View workflow status at: http://localhost:8080 (default Temporal UI)

### Metrics
All activities record metrics via Prometheus:
- Activity execution count (success/error)
- Activity duration
- Block processing rate (for batch workflows)

### Example Queries

```promql
# Batch processing rate
rate(dixmgr_activity_duration_seconds_sum{activity="ProcessBlockBatch"}[5m])

# Query execution errors
dixmgr_activity_execution_total{activity="ExecuteAndStoreNamedQuery", status="error"}

# Average activity duration
dixmgr_activity_duration_seconds_sum / dixmgr_activity_duration_seconds_count
```

## Migration from dixbatch/dixcron

### Before (Old Approach)

```bash
# Run batch manually
./dixbatch -conf conf.toml -relayChain polkadot -chain assethub

# Start cron service
systemctl start dixcron.service
```

### After (Temporal Approach)

```bash
# Start batch workflow
temporal workflow start \
  --task-queue dotidx-watcher \
  --type BatchWorkflow \
  --workflow-id wf.batch.polkadot.assethub \
  --input '{"RelayChain":"polkadot","Chain":"assethub",...}'

# Start cron workflows (hourly + daily)
temporal workflow start --type CronWorkflow --cron "0 * * * *" ...
temporal workflow start --type CronWorkflow --cron "0 0 * * *" ...
```

## Advantages Over Old Implementation

| Feature | dixbatch/dixcron | Temporal Services |
|---------|------------------|-------------------|
| **Observability** | Logs only | Temporal UI + Metrics |
| **Retries** | Manual | Automatic with backoff |
| **State** | Lost on crash | Persisted in Temporal |
| **Scheduling** | systemd/cron | Temporal cron |
| **Scalability** | Single process | Horizontal scaling |
| **Progress Tracking** | None | Workflow history |
| **Dependency Management** | Manual | Signal-based |
| **Long-running Jobs** | Memory issues | Continue-As-New |

## Performance Considerations

### Batch Workflow
- **MaxWorkers**: Balance between throughput and system load (recommended: 10-20)
- **BatchSize**: Larger batches are more efficient but may timeout (recommended: 50-100)
- **Continue-As-New**: Prevents workflow history from growing too large

### Cron Workflow
- **Hourly Schedule**: Lightweight, only updates current month
- **Daily Schedule**: Heavier, processes all historical data
- **Query Optimization**: Use indexed columns for date filtering

## Troubleshooting

### Batch Workflow Not Processing Blocks
1. Check Sidecar URL is accessible: `curl http://localhost:8080/blocks/head`
2. Verify database connection in activities
3. Check worker is running: `temporal workflow list`
4. View workflow history: `temporal workflow show --workflow-id wf.batch.polkadot.assethub`

### Cron Workflow Not Executing
1. Verify cron schedule syntax: https://crontab.guru/
2. Check if queries are registered: look for "Registered default queries" in logs
3. Ensure database info table has data: `SELECT * FROM chain.dotidx_database_info`

### Activity Failures
1. Check activity retry policy in workflow code
2. View activity errors in Temporal UI
3. Check metrics for error counts
4. Review worker logs for detailed error messages

## Future Enhancements

- [ ] Dynamic worker scaling based on workload
- [ ] Priority queues for urgent batch jobs
- [ ] Query result caching
- [ ] Batch workflow pause/resume
- [ ] Custom query registration via API
- [ ] Multi-region deployment support
