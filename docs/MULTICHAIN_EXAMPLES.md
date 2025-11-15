# Multi-Chain Utilities - Usage Examples

This document provides practical examples of using the multi-chain utilities (`app/multichain.js`) for working with data across multiple blockchain networks.

## Table of Contents

1. [Basic Iteration](#basic-iteration)
2. [Flattening Data](#flattening-data)
3. [Filtering](#filtering)
4. [Grouping and Aggregation](#grouping-and-aggregation)
5. [Error Handling](#error-handling)
6. [Real-World Examples](#real-world-examples)

## Data Structure

All multi-chain API endpoints return data in this nested format:

```javascript
{
  "polkadot": {
    "assethub": [
      { /* block data */ },
      { /* block data */ }
    ],
    "statemint": [
      { /* block data */ }
    ]
  },
  "kusama": {
    "assethub": [
      { /* block data */ }
    ]
  }
}
```

Structure: `Map<Relay, Map<Chain, Array<DataItem>>>`

## Basic Iteration

### Simple iteration over all items

```javascript
import { forEachMultiChain } from './multichain.js';

// Iterate over all blocks
forEachMultiChain(results, (relay, chain, block, index) => {
    console.log(`Block ${block.block_id} from ${relay}/${chain}`);
});
```

### With filtering

```javascript
// Only process blocks from polkadot
forEachMultiChain(
    results,
    (relay, chain, block) => {
        console.log(`Processing ${relay}/${chain} block ${block.block_id}`);
    },
    {
        filter: (relay, chain, block) => relay === 'polkadot'
    }
);
```

### With callbacks

```javascript
// Track progress as chains are processed
forEachMultiChain(
    results,
    (relay, chain, block) => {
        // Process each block
    },
    {
        onChainStart: (relay, chain, count) => {
            console.log(`Starting ${relay}/${chain} with ${count} blocks`);
        },
        onChainEnd: (relay, chain, count) => {
            console.log(`Finished ${relay}/${chain}`);
        }
    }
);
```

## Flattening Data

### Basic flattening (adds relay/chain metadata)

```javascript
import { flattenMultiChain } from './multichain.js';

// Flatten to single array
const allBlocks = flattenMultiChain(results);
// Result: [
//   { relay: 'polkadot', chain: 'assethub', block_id: '1000', ... },
//   { relay: 'polkadot', chain: 'statemint', block_id: '2000', ... },
//   ...
// ]
```

### With transformation

```javascript
// Extract only specific fields
const blockSummaries = flattenMultiChain(results, (relay, chain, block) => ({
    chainId: `${relay}/${chain}`,
    blockId: block.block_id,
    timestamp: block.timestamp,
    txCount: block.extrinsics?.length || 0
}));
```

### Filter while flattening

```javascript
// Flatten only recent blocks
const recentBlocks = flattenMultiChain(results, (relay, chain, block) => {
    const blockTime = new Date(block.timestamp);
    const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);

    if (blockTime > oneDayAgo) {
        return { relay, chain, ...block };
    }
    return null; // Skip this block
});
```

## Filtering

### Filter by relay or chain

```javascript
import { filterMultiChain } from './multichain.js';

// Only polkadot chains
const polkadotOnly = filterMultiChain(results, {
    relay: 'polkadot'
});

// Only assethub chains (across all relays)
const assethubOnly = filterMultiChain(results, {
    chain: 'assethub'
});

// Multiple relays
const filtered = filterMultiChain(results, {
    relay: ['polkadot', 'kusama']
});
```

### Check for data

```javascript
import { hasData, getChainsWithData } from './multichain.js';

// Quick check if any data exists
if (!hasData(results)) {
    console.log('No data found');
    return;
}

// Get list of chains with data
const chains = getChainsWithData(results);
// Result: [
//   { relay: 'polkadot', chain: 'assethub', count: 150 },
//   { relay: 'kusama', chain: 'assethub', count: 75 },
//   ...
// ]
```

## Grouping and Aggregation

### Count items

```javascript
import { countMultiChain } from './multichain.js';

const stats = countMultiChain(results);
console.log(`Total blocks: ${stats.totalItems}`);
console.log(`Chains queried: ${stats.chains}`);
console.log(`By relay:`, stats.byRelay);
// { polkadot: 200, kusama: 100 }
console.log(`By chain:`, stats.byChain);
// { 'polkadot/assethub': 150, 'polkadot/statemint': 50, ... }
```

### Group by custom key

```javascript
import { groupMultiChain } from './multichain.js';

// Group blocks by date
const blocksByDate = groupMultiChain(results, (relay, chain, block) => {
    const date = new Date(block.timestamp);
    return date.toISOString().split('T')[0]; // YYYY-MM-DD
});
// Result: {
//   '2024-01-15': [{ relay: 'polkadot', chain: 'assethub', ... }, ...],
//   '2024-01-16': [...],
//   ...
// }

// Group by block author
const blocksByAuthor = groupMultiChain(results, (relay, chain, block) => {
    return block.author_id || 'unknown';
});
```

### Reduce to single value

```javascript
import { reduceMultiChain } from './multichain.js';

// Count total extrinsics across all chains
const totalExtrinsics = reduceMultiChain(
    results,
    (total, relay, chain, block) => {
        return total + (block.extrinsics?.length || 0);
    },
    0 // initial value
);

// Build a map of latest blocks per chain
const latestBlocks = reduceMultiChain(
    results,
    (acc, relay, chain, block) => {
        const chainKey = `${relay}/${chain}`;
        if (!acc[chainKey] || block.block_id > acc[chainKey].block_id) {
            acc[chainKey] = block;
        }
        return acc;
    },
    {}
);
```

### Map over structure

```javascript
import { mapMultiChain } from './multichain.js';

// Add computed field to all blocks
const enrichedResults = mapMultiChain(results, (relay, chain, block) => ({
    ...block,
    chainId: `${relay}/${chain}`,
    txCount: block.extrinsics?.length || 0
}));
// Returns same nested structure with enriched blocks
```

## Error Handling

### Basic error tracking

```javascript
import { forEachMultiChain, MultiChainErrorCollector } from './multichain.js';

const errorCollector = new MultiChainErrorCollector();

forEachMultiChain(
    results,
    (relay, chain, block) => {
        // Process block - may throw error
        if (!block.timestamp) {
            throw new Error('Missing timestamp');
        }
    },
    {
        onError: (relay, chain, error) => {
            errorCollector.recordError(relay, chain, error);
        },
        onChainStart: (relay, chain, count) => {
            errorCollector.recordSuccess(relay, chain, count);
        }
    }
);

// Check for errors
if (errorCollector.hasErrors()) {
    console.log(errorCollector.getSummary());
    // "3/5 chains succeeded, 2 failed"

    console.log(errorCollector.getErrors());
    // [
    //   { relay: 'polkadot', chain: 'broken', error: 'Missing timestamp', timestamp: ... },
    //   ...
    // ]
}
```

### Display errors to user

```javascript
const errorCollector = new MultiChainErrorCollector();

// ... process data with error collector ...

// Display errors in UI
const errorDiv = document.getElementById('errors');
errorDiv.innerHTML = errorCollector.getErrorsHTML();
// Shows formatted notification with error details
```

## Real-World Examples

### Example 1: Extract Balance Events

```javascript
import { forEachMultiChain, MultiChainErrorCollector } from './multichain.js';

function extractBalances(results, address) {
    const balances = [];
    const errorCollector = new MultiChainErrorCollector();

    forEachMultiChain(
        results,
        (relay, chain, block) => {
            if (!block.extrinsics) return;

            block.extrinsics.forEach(extrinsic => {
                extrinsic.events.forEach(event => {
                    if (event?.method.pallet === 'balances') {
                        let amount = 0;

                        switch (event.method.method) {
                            case 'Transfer':
                                amount = parseFloat(event.data[2]);
                                if (address === event.data[0]) {
                                    amount = -amount;
                                }
                                break;
                            case 'Deposit':
                                amount = parseFloat(event.data[1]);
                                break;
                            case 'Withdraw':
                                amount = -parseFloat(event.data[1]);
                                break;
                        }

                        balances.push({
                            relay,
                            chain,
                            timestamp: block.timestamp,
                            blockId: block.block_id,
                            method: event.method.method,
                            amount: amount / 1e10 // Convert from planck
                        });
                    }
                });
            });
        },
        {
            onError: (relay, chain, error) => {
                errorCollector.recordError(relay, chain, error);
            },
            onChainStart: (relay, chain, count) => {
                if (count > 0) {
                    errorCollector.recordSuccess(relay, chain, count);
                }
            }
        }
    );

    return { balances, errorCollector };
}
```

### Example 2: Build Multi-Chain Statistics

```javascript
import { reduceMultiChain, getChainsWithData } from './multichain.js';

function buildChainStats(results) {
    // Count events by type across all chains
    const eventCounts = reduceMultiChain(
        results,
        (acc, relay, chain, block) => {
            if (!block.extrinsics) return acc;

            block.extrinsics.forEach(ext => {
                ext.events.forEach(event => {
                    const key = `${event.method.pallet}.${event.method.method}`;
                    acc[key] = (acc[key] || 0) + 1;
                });
            });

            return acc;
        },
        {}
    );

    // Get chain coverage
    const chains = getChainsWithData(results);
    const totalChains = chains.length;
    const totalBlocks = chains.reduce((sum, c) => sum + c.count, 0);

    return {
        totalChains,
        totalBlocks,
        eventCounts,
        chains
    };
}
```

### Example 3: Merge Results from Multiple Queries

```javascript
import { mergeMultiChain, sortMultiChain } from './multichain.js';

async function fetchAllDataForAddress(address) {
    // Fetch from multiple endpoints in parallel
    const [blocksResult, balancesResult, stakingResult] = await Promise.all([
        fetch(`/fe/address2blocks?address=${address}`).then(r => r.json()),
        fetch(`/fe/balances?address=${address}`).then(r => r.json()),
        fetch(`/fe/staking?address=${address}`).then(r => r.json())
    ]);

    // Merge all results
    const merged = mergeMultiChain(blocksResult, balancesResult, stakingResult);

    // Sort by timestamp within each chain
    const sorted = sortMultiChain(merged, (a, b) => {
        return new Date(b.timestamp) - new Date(a.timestamp);
    });

    return sorted;
}
```

### Example 4: Progressive Rendering

```javascript
import { forEachMultiChain } from './multichain.js';

async function renderBlocksProgressively(results) {
    const container = document.getElementById('blocks-container');
    let renderedCount = 0;

    forEachMultiChain(
        results,
        (relay, chain, block) => {
            // Render each block
            const blockEl = createBlockElement(relay, chain, block);
            container.appendChild(blockEl);
            renderedCount++;
        },
        {
            onChainStart: (relay, chain, count) => {
                // Add chain header
                const header = document.createElement('h3');
                header.textContent = `${relay}/${chain} (${count} blocks)`;
                container.appendChild(header);
            },
            onChainEnd: (relay, chain, count) => {
                console.log(`Rendered ${count} blocks from ${relay}/${chain}`);
            }
        }
    );

    console.log(`Total rendered: ${renderedCount} blocks`);
}
```

### Example 5: Filter and Export Data

```javascript
import { flattenMultiChain, filterMultiChain } from './multichain.js';

function exportToCSV(results, options = {}) {
    // Optionally filter first
    let data = results;
    if (options.relay || options.chain) {
        data = filterMultiChain(results, options);
    }

    // Flatten to array
    const flat = flattenMultiChain(data, (relay, chain, block) => ({
        chain: `${relay}/${chain}`,
        blockId: block.block_id,
        timestamp: block.timestamp,
        hash: block.hash,
        extrinsicCount: block.extrinsics?.length || 0
    }));

    // Convert to CSV
    const headers = Object.keys(flat[0]);
    const csv = [
        headers.join(','),
        ...flat.map(row => headers.map(h => row[h]).join(','))
    ].join('\n');

    return csv;
}

// Usage
const csv = exportToCSV(results, { relay: 'polkadot' });
downloadFile('polkadot-blocks.csv', csv);
```

## Performance Tips

1. **Use filtering early**: Filter data before flattening to reduce iterations
   ```javascript
   // Good
   const filtered = filterMultiChain(results, { relay: 'polkadot' });
   const flat = flattenMultiChain(filtered);

   // Less efficient
   const flat = flattenMultiChain(results);
   const filtered = flat.filter(item => item.relay === 'polkadot');
   ```

2. **Use reduce for single-pass aggregation**: More efficient than multiple iterations
   ```javascript
   // Good - single pass
   const stats = reduceMultiChain(results, (acc, relay, chain, block) => {
       acc.count++;
       acc.totalTx += block.extrinsics?.length || 0;
       return acc;
   }, { count: 0, totalTx: 0 });

   // Less efficient - multiple passes
   const count = countMultiChain(results).totalItems;
   const totalTx = flattenMultiChain(results)
       .reduce((sum, b) => sum + (b.extrinsics?.length || 0), 0);
   ```

3. **Use error collectors**: Track errors without throwing, allows partial success
   ```javascript
   const errorCollector = new MultiChainErrorCollector();
   // Process data...
   // Even if some chains fail, others succeed
   ```

## Summary

The multi-chain utilities provide:

- **Consistent patterns** for working with nested relay/chain/data structures
- **Error handling** for partial failures across chains
- **Flexible operations** (map, filter, reduce, group) while preserving metadata
- **Performance optimizations** through single-pass operations
- **Type safety** through consistent interfaces

Use these utilities whenever you need to process data from multiple chains to maintain consistency and reduce boilerplate code.
