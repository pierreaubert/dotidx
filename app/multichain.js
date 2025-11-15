/**
 * Multi-Chain Data Utilities
 *
 * Reusable utilities for working with multi-chain data structures.
 * All API endpoints return data in the format: {relay: {chain: [data]}}
 * These utilities provide consistent patterns for processing this nested structure.
 */

/**
 * Iterate over all items in a multi-chain result structure
 * Calls the callback for each relay/chain/item combination
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Function} callback - Function(relay, chain, item, index) to call for each item
 * @param {Object} options - Optional configuration
 * @param {Function} options.filter - Optional filter function(relay, chain, item) => boolean
 * @param {Function} options.onChainStart - Optional callback when starting a new chain
 * @param {Function} options.onChainEnd - Optional callback when finishing a chain
 * @param {Function} options.onError - Optional error handler(relay, chain, error)
 */
export function forEachMultiChain(results, callback, options = {}) {
    const { filter, onChainStart, onChainEnd, onError } = options;

    for (const [relay, chains] of Object.entries(results)) {
        if (!chains || typeof chains !== 'object') {
            continue;
        }

        for (const [chain, items] of Object.entries(chains)) {
            // Skip undefined or invalid chain data
            if (items == undefined || !Array.isArray(items)) {
                if (onError) {
                    onError(relay, chain, new Error('Invalid or missing chain data'));
                }
                continue;
            }

            if (onChainStart) {
                onChainStart(relay, chain, items.length);
            }

            try {
                items.forEach((item, index) => {
                    // Apply filter if provided
                    if (filter && !filter(relay, chain, item)) {
                        return; // Skip this item
                    }

                    callback(relay, chain, item, index);
                });
            } catch (error) {
                if (onError) {
                    onError(relay, chain, error);
                } else {
                    console.error(`Error processing ${relay}/${chain}:`, error);
                }
            }

            if (onChainEnd) {
                onChainEnd(relay, chain, items.length);
            }
        }
    }
}

/**
 * Flatten multi-chain results into a single array
 * Each item gets relay/chain metadata added
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Function} transform - Optional transform function(relay, chain, item) => transformedItem
 * @returns {Array} Flattened array with metadata
 */
export function flattenMultiChain(results, transform = null) {
    const flattened = [];

    forEachMultiChain(results, (relay, chain, item) => {
        let flatItem;
        if (transform) {
            flatItem = transform(relay, chain, item);
        } else {
            // Default: add relay/chain metadata
            flatItem = {
                relay,
                chain,
                ...item,
            };
        }

        if (flatItem !== null && flatItem !== undefined) {
            flattened.push(flatItem);
        }
    });

    return flattened;
}

/**
 * Filter multi-chain results by relay and/or chain
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Object} filters - Filter criteria
 * @param {string|string[]} filters.relay - Relay name(s) to include
 * @param {string|string[]} filters.chain - Chain name(s) to include
 * @returns {Object} Filtered results in same nested structure
 */
export function filterMultiChain(results, filters = {}) {
    const filtered = {};
    const relayFilter = Array.isArray(filters.relay) ? filters.relay : filters.relay ? [filters.relay] : null;
    const chainFilter = Array.isArray(filters.chain) ? filters.chain : filters.chain ? [filters.chain] : null;

    for (const [relay, chains] of Object.entries(results)) {
        // Filter by relay if specified
        if (relayFilter && !relayFilter.includes(relay)) {
            continue;
        }

        filtered[relay] = {};

        for (const [chain, items] of Object.entries(chains)) {
            // Filter by chain if specified
            if (chainFilter && !chainFilter.includes(chain)) {
                continue;
            }

            if (items && Array.isArray(items)) {
                filtered[relay][chain] = items;
            }
        }
    }

    return filtered;
}

/**
 * Count total items across all chains
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @returns {Object} Count statistics
 */
export function countMultiChain(results) {
    const stats = {
        totalItems: 0,
        byRelay: {},
        byChain: {},
        chains: 0,
    };

    forEachMultiChain(
        results,
        (relay, chain, item) => {
            stats.totalItems++;

            if (!stats.byRelay[relay]) {
                stats.byRelay[relay] = 0;
            }
            stats.byRelay[relay]++;

            const chainKey = `${relay}/${chain}`;
            if (!stats.byChain[chainKey]) {
                stats.byChain[chainKey] = 0;
            }
            stats.byChain[chainKey]++;
        },
        {
            onChainStart: () => {
                stats.chains++;
            },
        }
    );

    return stats;
}

/**
 * Group multi-chain data by a custom key
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Function} keyFn - Function(relay, chain, item) => key
 * @returns {Object} Grouped data {key: [items with metadata]}
 */
export function groupMultiChain(results, keyFn) {
    const grouped = {};

    forEachMultiChain(results, (relay, chain, item) => {
        const key = keyFn(relay, chain, item);

        if (!grouped[key]) {
            grouped[key] = [];
        }

        grouped[key].push({
            relay,
            chain,
            ...item,
        });
    });

    return grouped;
}

/**
 * Reduce multi-chain data to a single value
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Function} reducer - Function(accumulator, relay, chain, item) => newAccumulator
 * @param {*} initialValue - Initial accumulator value
 * @returns {*} Reduced value
 */
export function reduceMultiChain(results, reducer, initialValue) {
    let accumulator = initialValue;

    forEachMultiChain(results, (relay, chain, item) => {
        accumulator = reducer(accumulator, relay, chain, item);
    });

    return accumulator;
}

/**
 * Map multi-chain data, maintaining nested structure
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Function} mapFn - Function(relay, chain, item) => mappedItem
 * @returns {Object} Mapped results in same nested structure
 */
export function mapMultiChain(results, mapFn) {
    const mapped = {};

    for (const [relay, chains] of Object.entries(results)) {
        if (!chains || typeof chains !== 'object') {
            continue;
        }

        mapped[relay] = {};

        for (const [chain, items] of Object.entries(chains)) {
            if (!items || !Array.isArray(items)) {
                mapped[relay][chain] = [];
                continue;
            }

            mapped[relay][chain] = items.map((item) => mapFn(relay, chain, item));
        }
    }

    return mapped;
}

/**
 * Check if multi-chain results contain any data
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @returns {boolean} True if any chain has data
 */
export function hasData(results) {
    if (!results || typeof results !== 'object') {
        return false;
    }

    for (const chains of Object.values(results)) {
        if (!chains || typeof chains !== 'object') {
            continue;
        }

        for (const items of Object.values(chains)) {
            if (items && Array.isArray(items) && items.length > 0) {
                return true;
            }
        }
    }

    return false;
}

/**
 * Get list of all chains that have data
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @returns {Array} Array of {relay, chain, count} objects
 */
export function getChainsWithData(results) {
    const chains = [];

    forEachMultiChain(
        results,
        () => {}, // Don't need to process items
        {
            onChainStart: (relay, chain, count) => {
                if (count > 0) {
                    chains.push({ relay, chain, count });
                }
            },
        }
    );

    return chains;
}

/**
 * Merge multiple multi-chain result sets
 * Useful for combining results from different queries
 *
 * @param {...Object} resultSets - Multiple result objects to merge
 * @returns {Object} Merged results
 */
export function mergeMultiChain(...resultSets) {
    const merged = {};

    resultSets.forEach((results) => {
        if (!results || typeof results !== 'object') {
            return;
        }

        for (const [relay, chains] of Object.entries(results)) {
            if (!merged[relay]) {
                merged[relay] = {};
            }

            for (const [chain, items] of Object.entries(chains)) {
                if (!items || !Array.isArray(items)) {
                    continue;
                }

                if (!merged[relay][chain]) {
                    merged[relay][chain] = [];
                }

                // Concatenate items
                merged[relay][chain] = merged[relay][chain].concat(items);
            }
        }
    });

    return merged;
}

/**
 * Sort items within each chain
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @param {Function} compareFn - Compare function(a, b) => number
 * @returns {Object} Sorted results in same nested structure
 */
export function sortMultiChain(results, compareFn) {
    const sorted = {};

    for (const [relay, chains] of Object.entries(results)) {
        if (!chains || typeof chains !== 'object') {
            continue;
        }

        sorted[relay] = {};

        for (const [chain, items] of Object.entries(chains)) {
            if (!items || !Array.isArray(items)) {
                sorted[relay][chain] = [];
                continue;
            }

            sorted[relay][chain] = [...items].sort(compareFn);
        }
    }

    return sorted;
}

/**
 * Create a summary of multi-chain results for display
 *
 * @param {Object} results - Multi-chain results {relay: {chain: [items]}}
 * @returns {string} Human-readable summary
 */
export function summarizeMultiChain(results) {
    const stats = countMultiChain(results);
    const chains = getChainsWithData(results);

    if (stats.totalItems === 0) {
        return 'No data found across any chains';
    }

    const chainsList = chains.map((c) => `${c.relay}/${c.chain} (${c.count})`).join(', ');

    return `Found ${stats.totalItems} items across ${stats.chains} chains: ${chainsList}`;
}

/**
 * Handle errors in multi-chain processing
 * Useful for displaying which chains succeeded/failed
 */
export class MultiChainErrorCollector {
    constructor() {
        this.errors = [];
        this.successes = [];
    }

    /**
     * Record a chain error
     */
    recordError(relay, chain, error) {
        this.errors.push({
            relay,
            chain,
            error: error.message || error.toString(),
            timestamp: new Date(),
        });
    }

    /**
     * Record a chain success
     */
    recordSuccess(relay, chain, count) {
        this.successes.push({
            relay,
            chain,
            count,
            timestamp: new Date(),
        });
    }

    /**
     * Check if there were any errors
     */
    hasErrors() {
        return this.errors.length > 0;
    }

    /**
     * Get all errors
     */
    getErrors() {
        return this.errors;
    }

    /**
     * Get all successes
     */
    getSuccesses() {
        return this.successes;
    }

    /**
     * Get a summary message
     */
    getSummary() {
        const total = this.errors.length + this.successes.length;
        if (this.errors.length === 0) {
            return `All ${this.successes.length} chains queried successfully`;
        } else if (this.successes.length === 0) {
            return `All ${this.errors.length} chains failed`;
        } else {
            return `${this.successes.length}/${total} chains succeeded, ${this.errors.length} failed`;
        }
    }

    /**
     * Get HTML for displaying errors
     */
    getErrorsHTML() {
        if (this.errors.length === 0) {
            return '';
        }

        let html = '<div class="notification is-warning">';
        html += `<p><strong>Some chains failed to load:</strong></p><ul>`;
        this.errors.forEach((err) => {
            html += `<li>${err.relay}/${err.chain}: ${err.error}</li>`;
        });
        html += '</ul></div>';
        return html;
    }
}
