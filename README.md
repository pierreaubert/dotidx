# Polkadot Block Indexer

A command-line tool for fetching block data from a Polkadot archive node via Sidecar and storing it in a PostgreSQL database.

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

## Prework

- Find 4 free TB of disk ideally SSD. It also works on SATA but is sloooow. A mix is also working well.
- Prepare your storage
  - ZFS or not: if ZFS then have a look at `./scripts/prep_zfs.sh`
  - Create a `/dotidx` directory owned by the database owner
  - Database uses more that 1 tablespace to optimise for permanance and cost. It expect 4 fast tablespaces and 6 slow ones.
  - Create links in `/dotidx` for each tablespace: it should look like this:
```
total 8
4 drwxr-xr-x  2 pierre pierre 4096 Mar 13 10:04 .
4 drwxr-xr-x 27 root   root   4096 Mar 13 10:03 ..
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 fast0 -> /data/dot-node/pg/16/fast0
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 fast1 -> /data/dot-node/pg/16/fast1
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 fast2 -> /data/dot-node/pg/16/fast2
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 fast3 -> /data/dot-node/pg/16/fast3
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow0 -> /data/backup/dotidx/slow0
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow1 -> /data/backup/dotidx/slow1
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow2 -> /data/backup/dotidx/slow2
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow3 -> /data/backup/dotidx/slow3
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow4 -> /data/backup/dotidx/slow4
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow5 -> /data/backup/dotidx/slow5
```
  - If you only have 1 disk have all the links point to the same disk.
- Create a database:
  - See `./scripts/setup-postgres.sh` for inspiration
  - Note the setup if you use ZFS.
  - Test that it is working with psql and that user dotidx can create a table.
- Start an archive Node
  - See `./scripts/start-archive.sh` for inspiration
  - It takes a few days to get all the data for the Relay Chain
  - Parity provides dump to go faster if you have enough bandwidth to get them.
  - Test that it is working by connecting it to polkadot.js for ex.
- Start a set of Sidecar API servers
  - See `./scripts/setup-sidecar.sh` for inspiration
  - Test that they are working by running some queries: `curl http://localhost:10801/blocks/head` should return a json file.
- Start a reverse proxy that also do load balancing
  - See `./scripts/start-nginx.sh` for inspiration
  - Test that they are working by running some queries: `curl http://localhost:10800/blocks/head` should return the same json file.


## Usage

```bash
dotidx -start=1000 -end=2000 -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db" -workers 5 -batch 10
```

or if you want to index live blocks:

```bash
dotidx -live -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db" -workers 5 -batch 10
```

### Command Line Options

| Option         | Description                                           | Default |
|----------------|-------------------------------------------------------|---------|
| `-start`       | Start of the block range                              | 1       |
| `-end`         | End of the block range                                | 10      |
| `-chainreader` | Sidecar API URL (required)                            | -       |
| `-database`    | PostgreSQL connection URI (required)                  | -       |
| `-batch`       | Number of items to collect before writing to database | 100     |
| `-workers`     | Maximum number of concurrent workers                  | 5       |
| `-flush`       | Maximum time to wait before flushing data to database | 30s     |
| `-live`        | Index new blocks on the fly                           |         |

> **Note**: The application automatically adds `sslmode=disable` to the PostgreSQL connection URI if not already specified. If you need SSL, explicitly include `sslmode=require` or another appropriate SSL mode in your connection string.

You should see something like:
```
2025/03/11 17:56:24 workers.go:310: +-- Blocks -------------|------ Chain Reader ----|------- DBwriter -------------+
2025/03/11 17:56:24 workers.go:311: | #----#  b/s  b/s  b/s | Latency          Error |  tr/s Latency          Error |
2025/03/11 17:56:24 workers.go:312: |          1d   1h   5m | avg min max (ms)     % |       avg min max (ms)    %  |
2025/03/11 17:56:24 workers.go:313: +-----------------------|------------------------|------------------------------|
2025/03/11 17:56:24 workers.go:319: |  40201 48.5 48.5 55.8 |  21  10   74      0%   |  562.1    2   1  141      0%  |
2025/03/11 17:56:39 workers.go:319: |  42171 48.5 48.5 47.1 |  21  10   74      0%   |  564.6    2   1  141      0%  |
2025/03/11 17:56:54 workers.go:319: |  44221 48.5 48.5 48.9 |  21  10   74      0%   |  556.5    2   1  141      0%  |
2025/03/11 17:57:09 workers.go:319: |  46241 48.5 48.5 48.7 |  21  10   74      0%   |  558.9    2   1  141      0%  |
2025/03/11 17:57:24 workers.go:319: |  48441 48.7 48.7 49.7 |  21  10   74      0%   |  564.3    2   1  141      0%  |
2025/03/11 17:57:39 workers.go:319: |  50551 48.8 48.8 49.9 |  20  10   74      0%   |  566.4    2   1  141      0%  |
2025/03/11 17:57:54 workers.go:319: |  52561 48.8 48.8 49.9 |  20  10   74      0%   |  560.0    2   1  141      0%  |
2025/03/11 17:58:09 workers.go:319: |  54661 48.9 48.9 50.1 |  20  10   74      0%   |  559.0    2   1  141      0%  |
2025/03/11 17:58:24 workers.go:319: |  56861 49.1 49.1 50.4 |  20  10   74      0%   |  562.5    2   1  141      0%  |
2025/03/11 17:58:39 workers.go:319: |  59041 49.2 49.2 50.6 |  20  10   74      0%   |  565.4    2   1  141      0%  |
2025/03/11 17:58:54 workers.go:319: |  61231 49.3 49.3 50.8 |  20  10   74      0%   |  569.5    2   1  141      0%  |
2025/03/11 17:59:09 workers.go:319: |  63301 49.3 49.3 50.8 |  20  10   74      0%   |  567.7    2   1  141      0%  |
2025/03/11 17:59:24 workers.go:319: |  65431 49.4 49.4 50.8 |  20  10   74      0%   |  570.6    2   1  141      0%  |
2025/03/11 17:59:39 workers.go:319: |  67521 49.4 49.4 50.8 |  20  10   74      0%   |  568.4    2   1  141      0%  |
2025/03/11 17:59:54 workers.go:319: |  69561 49.4 49.4 50.7 |  20  10   74      0%   |  567.1    2   1  141      0%  |
2025/03/11 18:00:09 workers.go:319: |  71551 49.4 49.4 50.5 |  20  10   74      0%   |  564.8    2   1  141      0%  |
2025/03/11 18:00:24 workers.go:319: |  73721 49.5 49.5 50.6 |  20  10   74      0%   |  566.8    2   1  141      0%  |
2025/03/11 18:00:39 workers.go:319: |  75891 49.5 49.5 50.7 |  20  10   74      0%   |  570.5    2   1  141      0%  |
2025/03/11 18:00:54 workers.go:319: |  78021 49.6 49.6 50.7 |  20  10   74      0%   |  573.2    2   1  141      0%  |
2025/03/11 18:01:09 workers.go:319: |  80191 49.6 49.6 50.8 |  20  10   74      0%   |  575.8    2   1  141      0%  |
```
This machine can read ~50 blocks per second and write them to the database so roughly a week to get up to date with 25_000_000 blocks.
The speed at which the node can read the blocks is the limiting factor.

Notes:
- I dont understand yet why it is so slow.
- If you can put the database on a diffent set of disks it does help a bit.
- M2 SSD will thermal throttle hard if they are not properly cooled.

## Testing

```bash
# Run all tests
go test -v ./...

# Run integration tests with database
TEST_POSTGRES_URI="postgres://user:password@localhost:5432/testdb" go test -v ./...
```

## Next features

- integrate [go-substrate-rpc-client](https://github.com/centrifuge/go-substrate-rpc-client/blob/master/main_test.go) and see if it can replace Sidecar
- add another database support, easy with the interface. Options are ScyllaDB, Cassandra or FoundationDB.
- more testing for other parachains (currently only Relay Chain and StateMint are tested.
- add a frontend to the DB with some UI
- replace the pile of shell scripts with a proper installer


## License

Apache 2, see [LICENSE](LICENSE) file.
