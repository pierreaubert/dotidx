# Multi-Chain Data Fetching Pattern

## Overview

This document describes the pattern used in dotidx for fetching data for a single address across multiple blockchain networks (chains).

## Architecture Decision

**Decision**: Perform data joins/unions in the frontend, not in the database.

**Rationale**:
- Tables are partitioned by chain at the database level
- No intersection operations needed, only unions of data
- Doing joins in the DB provides no bandwidth savings (same data transferred)
- IO impact is identical whether joining in DB or FE
- **Minimizes CPU usage on the database** by offloading union operations to clients
- Allows for better error handling and partial results

## Data Flow

### 1. Backend: Parallel Per-Chain Queries

The backend (`cmd/dixfe/r_address.go`) fetches data from multiple chains concurrently:

```go
func (f *Frontend) getBlocksByAddress(address string, count, from, to string) (
    map[string]map[string][]dix.BlockData,
    error,
) {
    blocks := make(map[string]map[string][]dix.BlockData)
    var wg sync.WaitGroup

    // Query each chain in parallel using goroutines
    for relay := range f.config.Parachains {
        blocks[relay] = make(map[string][]dix.BlockData)
        for chain := range f.config.Parachains[relay] {
            wg.Add(1)
            go func() {
                defer wg.Done()
                // Query single chain-partitioned table
                blocks[relay][chain], err = f.getBlocksByAddressForChain(
                    relay, chain, address, count, from, to
                )
            }()
        }
    }
    wg.Wait()
    return blocks, nil
}
```

**Key Points**:
- Each chain query is independent and runs in its own goroutine
- Queries hit partitioned tables: `blocks_{relay}_{chain}`
- Results maintain chain isolation in nested structure
- All queries run in parallel for optimal performance

### 2. Response Structure

The API returns a nested structure preserving chain boundaries:

```json
{
  "polkadot": {
    "assethub": [
      { "block_id": "1000", "timestamp": "...", "extrinsics": [...] },
      { "block_id": "1001", "timestamp": "...", "extrinsics": [...] }
    ],
    "statemint": [
      { "block_id": "2000", "timestamp": "...", "extrinsics": [...] }
    ]
  },
  "kusama": {
    "assethub": [
      { "block_id": "3000", "timestamp": "...", "extrinsics": [...] }
    ]
  }
}
```

**Structure**: `Map<Relay, Map<Chain, Array<BlockData>>>`

This structure:
- Preserves the source chain for each piece of data
- Allows frontend to handle partial failures gracefully
- Enables chain-specific filtering and processing
- Maintains data isolation for security and consistency

### 3. Frontend: Union Operations

The frontend (`app/balances.js`) performs union operations to flatten the nested structure:

```javascript
function extractBalancesFromBlocks(results, address, balanceAt) {
    const balances = [];

    // Iterate through nested structure and flatten
    for (const [relay, chains] of Object.entries(results)) {
        for (const [chain, blocks] of Object.entries(chains)) {
            if (blocks == undefined) {
                continue; // Handle missing/failed chains gracefully
            }
            blocks.forEach((block) => {
                // Extract events and add to flat array
                // Preserve relay/chain metadata in each record
                balances.push({
                    relay: relay,
                    chain: chain,
                    address: address,
                    timestamp: timestamp,
                    amount: amount,
                    // ... other fields
                });
            });
        }
    }
    return balances;
}
```

**Key Operations**:
1. **Iterate**: Loop through relay → chain → blocks hierarchy
2. **Extract**: Pull relevant data from each block
3. **Flatten**: Combine into single array with metadata
4. **Preserve context**: Keep relay/chain information for each record

## Benefits

### 1. Database Efficiency
- **Reduced CPU**: Database only performs simple SELECT queries
- **Optimal IO**: Each query reads from single partition
- **Parallel execution**: Independent queries run concurrently
- **No cross-partition joins**: Avoids expensive database operations

### 2. Frontend Flexibility
- **Partial results**: Can display data even if some chains fail
- **Progressive rendering**: Can render results as they arrive
- **Client-side filtering**: Can filter by chain without new queries
- **Rich error handling**: Can show which chains succeeded/failed

### 3. Scalability
- **Add chains easily**: New chains just add more parallel queries
- **Horizontal scaling**: Database partitioning scales naturally
- **Load distribution**: CPU load moves from DB to clients
- **Network efficiency**: Same bandwidth as database-side join

## Implementation Guidelines

### Backend Best Practices

1. **Always use goroutines for per-chain queries**
   ```go
   for relay := range f.config.Parachains {
       for chain := range f.config.Parachains[relay] {
           wg.Add(1)
           go func(r, c string) {
               defer wg.Done()
               // Query chain-specific table
           }(relay, chain)
       }
   }
   wg.Wait()
   ```

2. **Handle errors per-chain, don't fail entire request**
   ```go
   if err != nil {
       log.Printf("Error querying %s/%s: %v", relay, chain, err)
       // Continue with other chains
   }
   ```

3. **Return nested structure consistently**
   - Always use `map[string]map[string][]DataType`
   - Preserve empty arrays for chains with no data
   - Don't flatten in the backend

### Frontend Best Practices

1. **Check for undefined/null before processing**
   ```javascript
   if (blocks == undefined || !Array.isArray(blocks)) {
       continue;
   }
   ```

2. **Preserve chain metadata in flattened records**
   ```javascript
   balances.push({
       relay: relay,
       chain: chain,
       // ... data fields
   });
   ```

3. **Use consistent iteration pattern**
   ```javascript
   for (const [relay, chains] of Object.entries(results)) {
       for (const [chain, items] of Object.entries(chains)) {
           // Process items
       }
   }
   ```

## API Endpoints Using This Pattern

- `/fe/address2blocks` - All blocks for an address across all chains
- `/fe/balances` - Balance events filtered per chain
- `/fe/staking` - Staking events filtered per chain

All return the same nested structure: `{relay: {chain: [data]}}`

## Performance Characteristics

- **Database queries**: O(chains) parallel queries
- **Network transfer**: O(total_data) - same as DB-side join
- **Frontend processing**: O(total_data) linear scan
- **Memory usage**: O(total_data) - need to hold all results

For typical usage (1 address, ~10 chains, ~1000 blocks total):
- Query time: ~50-200ms (parallel)
- Transfer size: ~500KB-2MB
- Frontend processing: ~10-50ms

## Future Optimizations

1. **Streaming responses**: Stream per-chain results as they complete
2. **Client-side caching**: Cache per-chain results independently
3. **Request batching**: Batch multiple address queries
4. **Lazy loading**: Load additional chains on demand
5. **WebSocket updates**: Real-time updates per chain
