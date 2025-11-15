# dotidx Documentation

This directory contains technical documentation for the dotidx project.

## Architecture Documentation

### [MULTICHAIN_PATTERN.md](MULTICHAIN_PATTERN.md)
Comprehensive guide to the multi-chain data fetching pattern used throughout dotidx.

**Topics covered:**
- Architecture decision: Why joins happen in the frontend
- Data flow from backend to frontend
- Response structure format
- Benefits and trade-offs
- Backend and frontend best practices
- Performance characteristics

**Read this if you want to:**
- Understand how multi-chain queries work
- Learn about the data structure returned by APIs
- Implement new multi-chain features
- Optimize multi-chain data processing

### [MULTICHAIN_EXAMPLES.md](MULTICHAIN_EXAMPLES.md)
Practical examples and code snippets for using the multi-chain utilities.

**Topics covered:**
- Basic iteration patterns
- Flattening nested data structures
- Filtering by relay/chain
- Grouping and aggregation
- Error handling and partial failures
- Real-world implementation examples

**Read this if you want to:**
- See concrete examples of multi-chain utilities
- Learn how to process multi-chain data
- Implement common patterns (filtering, grouping, etc.)
- Handle errors gracefully in multi-chain contexts

## Multi-Chain Utilities

The multi-chain utilities (`app/multichain.js`) provide a consistent API for working with data across multiple blockchain networks.

### Key Features

1. **Consistent Iteration**: Simplified loops over nested relay/chain/data structures
2. **Data Transformation**: Flatten, filter, map, and reduce operations
3. **Error Handling**: Track partial failures across chains
4. **Metadata Preservation**: Keep relay/chain context throughout processing
5. **Performance**: Optimized single-pass operations

### Quick Start

```javascript
import { flattenMultiChain, summarizeMultiChain } from './multichain.js';

// Fetch multi-chain data
const response = await fetch('/fe/address2blocks?address=...');
const results = await response.json();

// Log summary
console.log(summarizeMultiChain(results));
// "Found 150 items across 3 chains: polkadot/assethub (100), kusama/assethub (50)"

// Flatten to single array
const allBlocks = flattenMultiChain(results);
// [
//   { relay: 'polkadot', chain: 'assethub', block_id: '1000', ... },
//   { relay: 'kusama', chain: 'assethub', block_id: '2000', ... },
//   ...
// ]
```

See [MULTICHAIN_EXAMPLES.md](MULTICHAIN_EXAMPLES.md) for more examples.

## Testing

### JavaScript Unit Tests

A test suite for the multi-chain utilities is available at `app/test_multichain.html`.

**To run tests:**
1. Start the dixfe server: `./dixfe -config conf/your-config.toml`
2. Open `http://localhost:8080/test_multichain.html` in a browser
3. Check that all tests pass

**Test coverage includes:**
- Iteration and filtering
- Data transformation (flatten, map, reduce)
- Grouping and aggregation
- Error handling
- Edge cases (empty data, null values, etc.)

## API Endpoints Using Multi-Chain Pattern

All these endpoints return data in the format: `{relay: {chain: [data]}}`

### `/fe/address2blocks`
Get all blocks containing a specific address across all chains.

**Query parameters:**
- `address` (required): The address to search for
- `count` (optional): Maximum blocks per chain (default: 10)
- `from` (optional): Start timestamp
- `to` (optional): End timestamp

**Example:**
```bash
curl "http://localhost:8080/fe/address2blocks?address=5GrwvaEF5z..."
```

### `/fe/balances`
Get balance events for an address across all chains.

**Query parameters:**
- `address` (required): The address to search for
- `from` (optional): Start timestamp
- `to` (optional): End timestamp

**Example:**
```bash
curl "http://localhost:8080/fe/balances?address=5GrwvaEF5z..."
```

### `/fe/staking`
Get staking events for an address across all chains.

**Query parameters:**
- `address` (required): The address to search for
- `from` (optional): Start timestamp
- `to` (optional): End timestamp

## Data Structure Reference

### Multi-Chain Response Format

```typescript
type MultiChainResponse<T> = {
  [relay: string]: {
    [chain: string]: T[]
  }
}
```

**Example:**
```json
{
  "polkadot": {
    "assethub": [
      {
        "block_id": "1000",
        "timestamp": "2024-01-15T10:00:00Z",
        "hash": "0x...",
        "extrinsics": [...]
      }
    ],
    "statemint": [...]
  },
  "kusama": {
    "assethub": [...]
  }
}
```

### Block Data Structure

```typescript
type BlockData = {
  block_id: string;
  timestamp: string;
  hash: string;
  parent_hash: string;
  state_root: string;
  extrinsics_root: string;
  author_id: string;
  finalized: boolean;
  on_initialize: object;
  on_finalize: object;
  logs: object[];
  extrinsics: Extrinsic[];
}
```

## Implementation Guidelines

### When to Use Multi-Chain Pattern

Use the multi-chain pattern when:
- Querying data for a single entity (address, validator, etc.) across multiple chains
- Data exists in chain-partitioned tables
- No cross-chain joins are needed (only unions)
- You want to minimize database CPU usage

### When NOT to Use Multi-Chain Pattern

Don't use this pattern when:
- Querying a single specific chain (use direct chain query instead)
- Need complex joins across chains (consider different approach)
- Data is not partitioned by chain

### Best Practices

**Backend:**
1. Always query chains in parallel using goroutines
2. Handle errors per-chain, don't fail entire request
3. Return empty arrays for failed chains
4. Use mutex to protect shared data structures
5. Log success/failure counts for monitoring

**Frontend:**
1. Use multi-chain utilities for consistent patterns
2. Track errors with `MultiChainErrorCollector`
3. Display partial results even if some chains fail
4. Preserve relay/chain metadata in flattened data
5. Filter early to reduce processing time

## Performance Considerations

### Database Level
- Each chain query is independent and parallel
- Uses chain-specific partitioned tables for optimal IO
- No cross-partition joins required
- Scales horizontally as chains are added

### Network Level
- Single HTTP request for all chains
- Response size: O(total_data) - same as DB-side join
- Supports compression (gzip)

### Frontend Level
- Flattening operation: O(n) where n = total items
- Filtering: O(n) linear scan
- Grouping: O(n) single pass
- Use single-pass operations (reduce) when possible

## Contributing

When adding new multi-chain features:

1. Follow the established pattern (parallel backend queries, nested response)
2. Use multi-chain utilities in frontend code
3. Add error handling for partial failures
4. Update documentation with examples
5. Add tests to `test_multichain.html`

## Additional Resources

- [Main README](../README.md) - Project overview and setup
- [Configuration Guide](../conf/conf-simple.toml) - Example configuration
- [Database Schema](../dix/database.go) - Database layer implementation
