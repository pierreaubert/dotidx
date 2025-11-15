# E2E Test System for dotidx

This document describes the End-to-End (E2E) test system for the Polkadot Block Indexer (dotidx).

## Overview

The E2E test system validates the complete block indexing pipeline by:

1. **Fetching** blocks from Polkadot relay chain and AssetHub parachain (last 10 days)
2. **Indexing** all blocks into a SQLite database
3. **Running** verification queries to ensure data integrity
4. **Generating** a comprehensive test report

## Prerequisites

### 1. Running Substrate API Sidecar Instances

You need to have Substrate API Sidecar instances running for both chains:

- **Polkadot Relay Chain Sidecar**: `http://127.0.0.1:10800`
- **AssetHub Parachain Sidecar**: `http://127.0.0.1:10900`

#### Option A: Using Docker

```bash
# Polkadot relay chain sidecar
docker run -d --name polkadot-sidecar \
  -p 10800:8080 \
  -e SAS_SUBSTRATE_URL=wss://polkadot-rpc.dwellir.com \
  parity/substrate-api-sidecar:latest

# AssetHub parachain sidecar
docker run -d --name assethub-sidecar \
  -p 10900:8080 \
  -e SAS_SUBSTRATE_URL=wss://polkadot-asset-hub-rpc.polkadot.io \
  parity/substrate-api-sidecar:latest
```

#### Option B: Using Local Nodes

If you have local archive nodes running:

```bash
# Install substrate-api-sidecar
npm install -g @substrate/api-sidecar

# Run for Polkadot (adjust WS URL to your local node)
SAS_SUBSTRATE_URL=ws://127.0.0.1:9944 substrate-api-sidecar --port 10800

# Run for AssetHub (in another terminal)
SAS_SUBSTRATE_URL=ws://127.0.0.1:9945 substrate-api-sidecar --port 10900
```

### 2. Build the E2E Test Binary

```bash
# Build the e2e test command
make e2e

# Or build everything
make all
```

## Configuration

The E2E test configuration is located at `conf/conf-e2e-test.toml`. Key parameters:

```toml
[dotidx_db]
type = "sqlite"              # Database type (SQLite for tests)
data_dir = "./e2e-test/db"   # SQLite database location

[dotidx_batch]
max_workers = 4              # Number of concurrent workers
batch_size = 10              # Blocks per batch

[parachains.polkadot.polkadot]
sidecar_ip = "127.0.0.1"
sidecar_port = 10800         # Polkadot sidecar port

[parachains.polkadot.assethub]
sidecar_ip = "127.0.0.1"
sidecar_port = 10900         # AssetHub sidecar port
```

You can customize these values based on your setup.

## Running the E2E Tests

### Quick Start

```bash
# Run the complete E2E test suite
make test-e2e
```

### Manual Execution

```bash
# Run with default configuration
./bin/dixe2e -conf conf/conf-e2e-test.toml

# Run with verbose output
./bin/dixe2e -conf conf/conf-e2e-test.toml -verbose
```

## What Gets Tested

### 1. Block Indexing

The test system:
- Calculates the block range for the last 10 days based on average block times
  - Polkadot: ~6 second block time → ~144,000 blocks per 10 days
  - AssetHub: ~12 second block time → ~72,000 blocks per 10 days
- Fetches and indexes all blocks in parallel using worker pools
- Stores block data in SQLite with proper schema

### 2. Verification Queries

The following queries are executed to verify data integrity:

#### For Each Chain (Polkadot & AssetHub):

1. **Block Count Verification**
   - Ensures the expected number of blocks were indexed
   - SQL: `SELECT COUNT(*) FROM chain_blocks_polkadot_polkadot`

2. **Hash Uniqueness**
   - Verifies all block hashes are unique
   - SQL: `SELECT COUNT(DISTINCT hash) FROM chain_blocks_...`

3. **Timestamp Ordering**
   - Checks that timestamps are properly ordered by block ID
   - SQL: `SELECT block_id, created_at FROM chain_blocks_... ORDER BY block_id`

4. **Block Sequence Integrity**
   - Verifies no gaps exist in the block sequence
   - Compares indexed count with expected range

5. **Data Integrity**
   - Ensures required fields (hash, parent_hash, extrinsics) are populated
   - SQL: `SELECT COUNT(*) WHERE hash IS NOT NULL AND ...`

## Test Output

### Sample Output

```
======================================================================
           E2E Test: Polkadot Block Indexer (dotidx)
======================================================================
Test Configuration:
  - Database: SQLite
  - Chains: Polkadot relay chain + AssetHub parachain
  - Time Range: Last 10 days
  - Workers: 4
======================================================================

✓ Successfully connected to SQLite database

--- Initializing polkadot:polkadot ---
✓ Connected to Sidecar at http://127.0.0.1:10800
✓ Current head block: 25000000
✓ Block range calculated: 24856000 to 25000000 (144000 blocks, ~10 days)
✓ Database tables created

--- Initializing polkadot:assethub ---
✓ Connected to Sidecar at http://127.0.0.1:10900
✓ Current head block: 10000000
✓ Block range calculated: 9928000 to 10000000 (72000 blocks, ~10 days)
✓ Database tables created

======================================================================
                    Starting Block Indexing
======================================================================

--- Indexing polkadot:polkadot ---
  Progress: 14400/144000 blocks (10.0%) | 120.5 blocks/sec | 0 errors
  Progress: 28800/144000 blocks (20.0%) | 118.3 blocks/sec | 0 errors
  ...
✓ Completed indexing polkadot in 20m15s (118.52 blocks/sec)

--- Indexing polkadot:assethub ---
  Progress: 7200/72000 blocks (10.0%) | 95.2 blocks/sec | 0 errors
  ...
✓ Completed indexing assethub in 12m30s (96.00 blocks/sec)

======================================================================
                    Running Verification Queries
======================================================================

✓ Block Count [polkadot]: Indexed 144000 blocks
✓ Hash Uniqueness [polkadot]: All block hashes are unique
✓ Timestamp Ordering [polkadot]: Timestamps are correctly ordered
✓ Block Sequence [polkadot]: Block sequence from 24856000 to 25000000 is complete
✓ Data Integrity [polkadot]: All required fields are populated
✓ Block Count [assethub]: Indexed 72000 blocks
✓ Hash Uniqueness [assethub]: All block hashes are unique
✓ Timestamp Ordering [assethub]: Timestamps are correctly ordered
✓ Block Sequence [assethub]: Block sequence from 9928000 to 10000000 is complete
✓ Data Integrity [assethub]: All required fields are populated

======================================================================
                         E2E Test Report
======================================================================

Summary:
  Total Duration: 32m45s
  Total Blocks Indexed: 216000
  Total Errors: 0
  Average Rate: 109.92 blocks/sec

Chain Details:
  polkadot:polkadot
    - Block Range: 24856000 to 25000000
    - Blocks Indexed: 144000
    - Errors: 0
    - Duration: 20m15s
    - Rate: 118.52 blocks/sec
  polkadot:assethub
    - Block Range: 9928000 to 10000000
    - Blocks Indexed: 72000
    - Errors: 0
    - Duration: 12m30s
    - Rate: 96.00 blocks/sec

Verification Tests:
  Passed: 10
  Failed: 0

======================================================================

✓ All tests passed!
```

## Database Schema

The E2E test creates the following SQLite tables:

```sql
-- Polkadot relay chain blocks
CREATE TABLE chain_blocks_polkadot_polkadot (
    block_id        INTEGER NOT NULL,
    created_at      TEXT NOT NULL,
    hash            TEXT NOT NULL,
    parent_hash     TEXT NOT NULL,
    state_root      TEXT NOT NULL,
    extrinsics_root TEXT NOT NULL,
    author_id       TEXT NOT NULL,
    finalized       INTEGER NOT NULL,
    on_initialize   TEXT,
    on_finalize     TEXT,
    logs            TEXT,
    extrinsics      TEXT,
    PRIMARY KEY (block_id, created_at)
);

-- AssetHub parachain blocks
CREATE TABLE chain_blocks_polkadot_assethub (
    -- Same schema as above
);

-- Address-to-block mappings
CREATE TABLE chain_address2blocks_polkadot_polkadot (...);
CREATE TABLE chain_address2blocks_polkadot_assethub (...);
```

## Troubleshooting

### Issue: "Sidecar service test failed"

**Solution**: Ensure Substrate API Sidecar instances are running and accessible:

```bash
# Test Polkadot sidecar
curl http://127.0.0.1:10800/blocks/head

# Test AssetHub sidecar
curl http://127.0.0.1:10900/blocks/head
```

### Issue: "Failed to ping database"

**Solution**: Ensure the database directory exists and is writable:

```bash
mkdir -p ./e2e-test/db
```

### Issue: Slow indexing performance

**Solution**: Adjust worker count and batch size in `conf/conf-e2e-test.toml`:

```toml
[dotidx_batch]
max_workers = 8    # Increase for more parallelism
batch_size = 20    # Increase for larger batches
```

### Issue: "Cannot get block" errors

**Solution**: This may indicate:
- Network connectivity issues with Sidecar
- Sidecar is connected to a node that's still syncing
- Block pruning on the underlying node (need archive node)

Ensure you're using **archive nodes** with full block history.

## Performance Expectations

Based on typical hardware configurations:

| Hardware | Expected Rate | Time for 10 days |
|----------|--------------|------------------|
| 4-core CPU, SSD | ~100 blocks/sec | ~35 minutes |
| 8-core CPU, SSD | ~200 blocks/sec | ~18 minutes |
| 16-core CPU, NVMe | ~400 blocks/sec | ~9 minutes |

**Note**: Actual performance depends on:
- Sidecar response times
- Network latency
- Disk I/O performance
- Number of workers configured

## Cleanup

To remove test data:

```bash
# Remove the entire e2e test directory
rm -rf ./e2e-test

# Or just the database
rm -rf ./e2e-test/db/*.db
```

## CI/CD Integration

To integrate E2E tests into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Start Sidecar Services
  run: |
    docker run -d -p 10800:8080 \
      -e SAS_SUBSTRATE_URL=wss://polkadot-rpc.dwellir.com \
      parity/substrate-api-sidecar:latest
    docker run -d -p 10900:8080 \
      -e SAS_SUBSTRATE_URL=wss://polkadot-asset-hub-rpc.polkadot.io \
      parity/substrate-api-sidecar:latest
    sleep 30  # Wait for sidecars to be ready

- name: Run E2E Tests
  run: make test-e2e
```

## Advanced Configuration

### Custom Block Range

To test a specific block range instead of "last 10 days", modify the code in `cmd/dixe2e/dixe2e.go`:

```go
// Replace automatic calculation with fixed range
chainCfg.StartBlock = 24000000
chainCfg.EndBlock = 24001000  // Test 1000 blocks
```

### Testing Additional Chains

Add more chains in the configuration and update the `chains` slice in `dixe2e.go`:

```go
chains := []*ChainConfig{
    {RelayChain: "polkadot", Chain: "polkadot", AvgBlockTime: polkadotBlockTime},
    {RelayChain: "polkadot", Chain: "assethub", AvgBlockTime: assetHubBlockTime},
    {RelayChain: "polkadot", Chain: "hydration", AvgBlockTime: 12 * time.Second},
    // Add more chains...
}
```

## Exit Codes

- `0`: All tests passed
- `1`: One or more verification queries failed

## Support

For issues or questions:
- Check the [main README](README.md)
- Review the [documentation](docs/)
- Open an issue on GitHub
