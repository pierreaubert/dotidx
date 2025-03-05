# Polkadot Block Indexer

A command-line tool for fetching block data from a Polkadot sidecar API and storing it in a PostgreSQL database.

## Features

- Fetches block data from a Polkadot sidecar API
- Stores data in a PostgreSQL database
- Supports concurrent processing of multiple blocks
- Configurable batch size and worker count
- Graceful shutdown on interrupt
- Performance metrics for API calls

## Requirements

- Go 1.18 or higher
- PostgreSQL database

## Installation

```bash
go get github.com/pierreaubert/polidx
```

## Usage

```bash
polidx -start=1000 -end=2000 -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db"
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-start` | Start of the block range | 1 |
| `-end` | End of the block range | 10 |
| `-sidecar` | Sidecar API URL (required) | - |
| `-postgres` | PostgreSQL connection URI (required) | - |
| `-batch` | Number of items to collect before writing to database | 100 |
| `-workers` | Maximum number of concurrent workers | 5 |
| `-flush` | Maximum time to wait before flushing data to database | 30s |

> **Note**: The application automatically adds `sslmode=disable` to the PostgreSQL connection URI if not already specified. If you need SSL, explicitly include `sslmode=require` or another appropriate SSL mode in your connection string.

### Example

```bash
polidx -start=1 -end=100 -sidecar=https://example.com/sidecar -postgres="postgres://user:password@localhost:5432/dbname" -batch=50 -workers=10 -flush=1m
```

## Database Schema

The application creates a PostgreSQL table with the following schema:

```sql
CREATE TABLE IF NOT EXISTS blocks_Polkadot_Polkadot (
    block_id INTEGER PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    hash VARCHAR(255),
    parenthash VARCHAR(255),
    stateroot VARCHAR(255),
    extrinsicsroot VARCHAR(255),
    authorid VARCHAR(255),
    finalized BOOLEAN,
    oninitialize JSONB,
    onfinalize JSONB,
    logs JSONB,
    extrinsics JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

The `block_id` field corresponds to the block number from the blockchain and serves as the primary key.

## Performance Metrics

The application now tracks and displays performance metrics for sidecar API calls. These metrics are printed every time data is written to the database and at the end of execution. The metrics include:

- Total number of API calls
- Number of failed calls
- Average latency
- Minimum latency
- Maximum latency
- Success rate

Example output:

```
Sidecar API Call Statistics:
  Total calls: 100
  Failed calls: 2
  Average latency: 245.32ms
  Minimum latency: 120.45ms
  Maximum latency: 890.12ms
  Success rate: 98.00%
```

These metrics can help you monitor the performance of the sidecar API and adjust your worker count and batch size accordingly.

## Testing

```bash
# Run all tests
go test -v ./...

# Run integration tests with database
TEST_POSTGRES_URI="postgres://user:password@localhost:5432/testdb" go test -v ./...
```

## License

MIT
