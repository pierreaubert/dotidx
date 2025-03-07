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

## Design

<img src="./docs/diagram/readme_seq.png" alt="sequence diagram" width="600">

## Installation

```bash
go get github.com/pierreaubert/dotidx
```

## Usage

```bash
dotidx -start=1000 -end=2000 -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db"
```

or if you want to index live blocks:

```bash
dotidx -live -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db"
```

### Command Line Options

| Option      | Description                                           | Default |
|-------------|-------------------------------------------------------|---------|
| `-start`    | Start of the block range                              | 1       |
| `-end`      | End of the block range                                | 10      |
| `-sidecar`  | Sidecar API URL (required)                            | -       |
| `-postgres` | PostgreSQL connection URI (required)                  | -       |
| `-batch`    | Number of items to collect before writing to database | 100     |
| `-workers`  | Maximum number of concurrent workers                  | 5       |
| `-flush`    | Maximum time to wait before flushing data to database | 30s     |
| `-live`     | Index new blocks on the fly                           |         |

> **Note**: The application automatically adds `sslmode=disable` to the PostgreSQL connection URI if not already specified. If you need SSL, explicitly include `sslmode=require` or another appropriate SSL mode in your connection string.

## Testing

```bash
# Run all tests
go test -v ./...

# Run integration tests with database
TEST_POSTGRES_URI="postgres://user:password@localhost:5432/testdb" go test -v ./...
```

## License

Apache 2, see [LICENSE](LICENSE) file.
