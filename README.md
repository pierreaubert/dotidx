# Polkadot Block Indexer

A utility for fetching block data from a Polkadot archive node via Sidecar and storing the data in a PostgreSQL database. It is relatively fast only limited by the speed of your disks. It also supports concurrent processing of multiple blocks.

Quality is currently *beta*. Contributions are very welcome!

## Features

- Fetches block data from a Polkadot parachain via a sidecar API
- Stores data in a PostgreSQL database
- Supports concurrent processing of multiple blocks
- Graceful shutdown on interrupt

## Requirements

- Go 1.20 or higher
- PostgreSQL database version 16 or higher

## Design

<img src="./docs/diagram/readme_seq.png" alt="sequence diagram" width="600">

## Installation

```bash
go get github.com/pierreaubert/dotidx
make
```

## Prework

Until the `dixmgr` is operational:

- Find 4 free TB of disk ideally on SSD disks. It also works on SATA but is sloooow. A mix is also working well. The more disks you have the faster this will be.
  - With 2 SATA disks, indexer run around 25 blocks per second.
  - With 4 NVME disks, indexer run around 300 blocks per second.
- Prepare your storage
  - ZFS or not: if ZFS then have a look at `./scripts/prep_zfs.sh`
  - Create a `/dotidx` directory owned by the database owner
  - Database uses more that 1 tablespace to optimise for permanance and cost. It expect 4 fast tablespaces and 6 slow ones.
  - Create links in `/dotidx` for each tablespace: it should look like this where each directory points to a different disk or partition. If you only have 1 disk, point all the links to the same disk.
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
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow3 -> /data/media1/dotidx/slow3
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow4 -> /data/media2/dotidx/slow4
0 lrwxrwxrwx  1 pierre pierre   26 Mar 13 10:04 slow5 -> /data/media3/dotidx/slow5
```
- Create a database:
  - See `./scripts/setup-postgres.sh` for inspiration
  - Note the setup if you use ZFS.
  - Test that it is working with psql and that user dotidx can create a table.
- Start an archive Node
  - See `./scripts/start-archive.sh` for inspiration
  - It takes a few days to get all the data for the Relay Chain
  - Parity provides dump to go faster if you have enough bandwidth to get them.
  - Test that it is working by connecting it to via polkadot.js for ex.
- Start a set of Sidecar API servers
  - See `./scripts/setup-sidecar.sh` for inspiration
  - Test that they are working by running some queries: `curl http://localhost:10801/blocks/head` should return a json file.
- Start a reverse proxy that also do load balancing
  - See `./scripts/start-nginx.sh` for inspiration
  - Test that they are working by running some queries: `curl http://localhost:10800/blocks/head` should return the same json file.

## Strategy to optimise for time

1. If your archive is on a SATA disks, make a copy on an SSD disk. It will take a few hours to make the copy but then indexing will be faster.
2. If you want an index mostly for yourself, you can let the database on SATA disks. If you plan to have a lot of requests, you can move some tablespace to an SSD and the index should definitively be on SSD.
3. You can delete the archive node on SSD when indexing is finished.
4. If you need to restart from scratch the indexer will produce monthly dumped that the database can restart from and it is significantly faster to start from dumps than loading the blocks from the archive node.


## Usage

The system built 5 binaries:

- `dixbatch` : pull large amount of blocks into the database.
- `dixlive`: can pull the head of 1 or many chains into the database (and run continously).
- `dixfe`: a web frontend to demonstrate how to use the data in the database.
- `dixcron`: a cron system running long range queries on the database.
- `dixmgr`: a service that launches and monitor all the various services (unrelease yet).

```bash
dotidx -start=1000 -end=2000 -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db" -workers 5 -batch 10
```

or if you want to index live blocks:

```bash
dotidx -live -sidecar=http://localhost:8080 -postgres="postgres://user:pass@localhost:5432/db" -workers 5 -batch 10
```

## Interface

```bash
dotfe -postgres="postgres://user:pass@localhost:5432/db"
```

The web API is available at 'http://localhost:8080'

### Command Line Options

| Option         | Description                                           | Default |
|----------------|-------------------------------------------------------|---------|
| `-start`       | Start of the block range                              | 1       |
| `-end`         | End of the block range                                | HeadID  |
| `-chainreader` | Sidecar API URL (required)                            | -       |
| `-database`    | PostgreSQL connection URI (required)                  | -       |
| `-batch`       | Number of items to collect before writing to database | 10      |
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
A mini pc machine can read ~30 blocks per second and write them to the database so roughly one week to get up to date with 25_000_000 blocks. With a larger machine (32 CPUs, 256GB RAM, 8x 1TB NVMe SSD) the indexer took 20h to get up to date. The speed at which the node can read the blocks is the limiting factor.

Notes:
- If you can put the database on a diffent set of disks it does help.
- M2 SSD will thermal throttle hard if they are not properly cooled.

## Testing

```bash
# Run all tests
go test -v ./...

# Run integration tests with database
TEST_POSTGRES_URI="postgres://user:password@localhost:5432/testdb" go test -v ./...
```

## Next features

- see [first project on GH](https://github.com/users/pierreaubert/projects/2)

## License

Apache 2, see [LICENSE](LICENSE) file.
