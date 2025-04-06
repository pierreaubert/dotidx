# Polkadot Block Indexer

A utility for fetching block data from a Polkadot archive node via Sidecar and storing the data in a PostgreSQL database. It is relatively fast only limited by the speed of your disks. It also supports concurrent processing of multiple blocks.

Quality is currently *beta*. Contributions are very welcome!

## Features

- Fetches block data from a Polkadot parachain via a sidecar API
- Stores data into a PostgreSQL database and can use multiple disks sata and ssd.
- Supports concurrent processing of multiple blocks
- Demo website at [dotidx.xyz](https://dev.dotidx.xyz/index.html)

## Requirements

- Go 1.20 or higher
- PostgreSQL database version 16 or higher
- Lots of disk space (>= 20TB to index a subset of the parachains)

## Design

<img src="./docs/diagram/readme_seq.png" alt="sequence diagram" width="600">

## Installation

```bash
go get github.com/pierreaubert/dotidx
make
```

## Prework

Until the `dixmgr` is fully operational:

- Find 6 free TB of disk ideally on SSD disks. It also works on SATA but is sloooow. A mix is also working well. The more disks you have the faster this will be.
  - With 2 SATA disks, indexer run around 25 blocks per second.
  - With 4 NVME disks, indexer run around 300 blocks per second.
- Prepare your storage
  - ZFS or not: if ZFS then have a look at `./scripts/prep_zfs.sh`
  - Create a `/dotidx` directory owned by the database owner
  - Database uses more that 1 tablespace to optimise for performance and cost. It expect 4 fast tablespaces and 6 slow ones.
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

The system build 5 binaries:

- `dixmgr`: a service that launches and monitor all the various services.
- `dixbatch` : pull large amount of blocks into the database.
- `dixlive`: can pull the head of 1 or many chains into the database (and run continously).
- `dixfe`: a web frontend to demonstrate how to use the data in the database.
- `dixcron`: a cron system running long range queries on the database.

### Configuration management

There is a main configuration file with a TOML syntax. There are two examples:
1. a simple one with all the services on one machine
2. a more complex one where each components can be on a different server.

The toml file is processed by `dixmgr` and generates configurations for a set of software that are required to work together:
- a database Postgres with a connection pooler
- a set of Polkadot nodes one per parachain and one for the relay chain
- a set of Sidecar frontends per node
- a Nginx reverse proxy
- a batch indexer per node
- a live indexer
- a frontend
- a monitoring system with
  - Prometheus
  - Grafana
  - Postgres exporter
  - Node exporter
  - Nging exporter

The current supported version is based on systemd. A docker or vagrant configuration can easily be build if needed. Setting up helm for K8s is also doable.

**Do not edit the generated files! They are overriden by the configuration manager.**

### Blocks ingestion

For example, if you want to ingest assethub on Polkadot:
```bash
dixbatch -conf conf/conf-simple.toml -relayChain polkadot -chain assethub
```

```
A mini pc machine can read ~30 blocks per second and write them to the database so roughly one week to get up to date with 25_000_000 blocks. With a larger machine (32 CPUs, 256GB RAM, 8x 1TB NVMe SSD) the indexer took 20h to get up to date. The speed at which the node can read the blocks is the limiting factor.

Notes:
- If you can put the database on a diffent set of disks it does help.
- M2 SSD will thermal throttle hard if they are not properly cooled. Look at the monitoring dashboard in Grafana to see what the temperature is.

### Continous ingestion of head blocks

```bash
dotlive -conf conf/conf-simple.toml
```

### Cron

Run some queries continuously in the background.

```bash
dotcron -conf conf/conf-simple.toml
```

### Frontend

```bash
dotfe -conf conf/conf-simple.toml
```

The web API is available at 'http://localhost:8080' in development mode. It does have a demo site.
The frontend does proxy duty for the static content and for sidecar which is convenient in dev mode.

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
