import { updateIcons, updateFooter, updateNav, updateSearchBlocks } from './components.js';
import { escapeHtml, highlightAddressMatches } from './misc.js';

function renderBlockContent(content, searchAddress) {
    if (content === null || content === undefined) {
        return '<span class="has-text-grey">null</span>';
    }

    if (Array.isArray(content)) {
        if (content.length === 0) {
            return '<span class="has-text-grey">(empty array)</span>';
        }

        let html = '<table class="table is-fullwidth is-bordered is-narrow nested-table">';
        html += '<thead><tr><th>Index</th><th>Value</th></tr></thead>';
        html += '<tbody>';

        content.forEach((item, index) => {
            html += `<tr><td>${index}</td><td>${renderBlockContent(item, searchAddress)}</td></tr>`;
        });

        html += '</tbody></table>';
        return html;
    }

    if (typeof content === 'object') {
        if (Object.keys(content).length === 0) {
            return '<span class="has-text-grey">(empty object)</span>';
        }

        let html = '<table class="table is-fullwidth is-bordered is-narrow nested-table">';
        html += '<thead><tr><th>Property</th><th>Value</th></tr></thead>';
        html += '<tbody>';

        Object.entries(content).forEach(([key, value]) => {
            html += `<tr><td>${key}</td><td>${renderBlockContent(value, searchAddress)}</td></tr>`;
        });

        html += '</tbody></table>';
        return html;
    }

    if (typeof content === 'string') {
        const escapedContent = escapeHtml(content);
        const highlightedContent = highlightAddressMatches(escapedContent, searchAddress);

        if (content.length > 50) {
            return `<span class="is-family-monospace break-word">${highlightedContent}</span>`;
        }
        return `<span class="is-family-monospace">${highlightedContent}</span>`;
    }

    return String(content);
}

async function fetchBlocks() {
    const resultDiv = document.getElementById('blocks-result');
    const dataDiv = document.getElementById('blocks-data');

    const searchInput = document.getElementById('search-block');
    if (!searchInput || !searchInput.value) {
        dataDiv.innerHTML = '<p>No blockID, No block.</p>';
        return;
    }

    const relaychain = document.getElementById('search-block-relaychain');
    if (!relaychain || !relaychain.value) {
        dataDiv.innerHTML = '<p>No blockID, No block.</p>';
        return;
    }
    const chain = document.getElementById('search-block-chain');
    if (!chain || !chain.value) {
        dataDiv.innerHTML = '<p>No blockID, No block.</p>';
        return;
    }
    const blockid = searchInput.value.trim();
    if (!blockid) {
        dataDiv.innerHTML = '<p>No blockID, No block.</p>';
        return;
    }
    const id = parseInt(blockid);
    if (isNaN(id) || id < 1) {
        dataDiv.innerHTML = '<p>blockID is not a positive integer!</p>';
        return;
    }

    const response = await fetch(`/fe/${relaychain.value}/${chain.value}/blocks/${blockid}`);
    if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
    }
    const data = await response.json();

    resultDiv.classList.remove('is-hidden');

    if (data) {
        if (data.extrinsics && typeof data.extrinsics === 'object') {
            const filteredExtrinsics = {};
            Object.entries(data.extrinsics).forEach(([_, extrinsics]) => {
                if (extrinsics?.method.pallet) {
                    const palletName = extrinsics.method.pallet;
                    if (palletName !== 'paraInherent') {
                        filteredExtrinsics[palletName] = extrinsics;
                    }
                }
            });
            data.extrinsics = filteredExtrinsics;
        }
        ensureBlockStyles();
        renderBlockPage(data);
    } else {
        dataDiv.innerHTML = '<p>No blocks found for this number.</p>';
    }
}

// Function to ensure block styles are applied
async function ensureBlockStyles() {
    if (!document.getElementById('block-styles')) {
        const style = document.createElement('style');
        style.id = 'block-styles';
        style.textContent = `
            .nested-table {
                margin-bottom: 0 !important;
            }
            .nested-table th, .nested-table td {
                padding: 0.3em 0.5em !important;
            }
            .nested-table .nested-table {
                font-size: 0.95em;
                border: 1px solid #eee;
            }
            .block-number {
                font-weight: bold;
                width: 120px;
            }
            .break-word {
                word-break: break-all;
            }
            .block-row td {
                vertical-align: top;
            }
            .address-highlight {
                background-color: #ffff00; /* Fluorescent yellow */
                font-weight: bold;
                padding: 1px 2px;
            }
            .pagination {
                display: flex;
                justify-content: center;
                margin-top: 20px;
                margin-bottom: 10px;
            }
            .pagination button {
                margin: 0 5px;
            }
            .pagination-info {
                margin: 0 10px;
                line-height: 2.25em;
            }
        `;
        document.head.appendChild(style);
    }
}

// Function to render the current block page
function renderBlockPage(block) {
    const dataDiv = document.getElementById('blocks-data');
    const searchInput = document.getElementById('search-block');

    let html = '';

    // Add block table
    html += '<table class="table is-fullwidth is-hoverable result-table">';
    html += '<thead><tr><th>Block</th><th>Content</th></tr></thead>';
    html += '<tbody>';

    const blockContent = {};
    Object.entries(block).forEach(([key, value]) => {
        if (
            value !== null &&
            value !== undefined &&
            !(Array.isArray(value) && value.length === 0) &&
            !(typeof value === 'object' && Object.keys(value).length === 0)
        ) {
            blockContent[key] = value;
        }
    });

    const blockNumber = block.block_number || block.number || 'N/A';
    const blockKeys = Object.keys(blockContent).sort((a, b) => {
        // Always put block_number and hash first
        if (a === 'block_number' || a === 'number') return -1;
        if (b === 'block_number' || b === 'number') return 1;
        if (a === 'hash') return -1;
        if (b === 'hash') return 1;
        return a.localeCompare(b);
    });

    if (blockKeys.length > 0) {
        // Main block row
        html += `<tr class="block-row">`;
        html += `<td class="block-number">${blockNumber}</td>`;

        // Content column with nested table
        html += '<td>';
        html += '<table class="table is-fullwidth is-bordered is-narrow nested-table">';
        html += '<tbody>';

        blockKeys.forEach((key) => {
            if (key === 'block_number' || key === 'number') return;
            const value = blockContent[key];
            html += `
<tr>
  <td width="150">${key}</td>
  <td>${renderBlockContent(value, searchInput.value.trim())}</td>
</tr>`;
        });

        html += '</tbody></table>';
        html += '</td>';
        html += '</tr>';
    }

    html += '</tbody></table>';

    html += '</div>';

    dataDiv.innerHTML = html;
}

function updateUrl() {
    const searchInput = document.getElementById('search-block');
    const blockid = searchInput.value.trim();
    if (!blockid) {
        return;
    }
    let newUrl = `/blocks.html?blockid=${blockid}`;
    window.history.pushState({}, '', newUrl);
}

function updateFromUrl() {
    const urlParams = new URLSearchParams(window.location.search);
    const blockid = urlParams.get('blockid');
    const searchInput = document.getElementById('search-block');
    searchInput.value = blockid;
    const actionButton = document.getElementById('action-button');
    actionButton.click();
}

async function initBlocks() {
    await updateIcons();
    await updateNav();
    await updateFooter();
    await updateSearchBlocks();

    const actionButton = document.getElementById('action-button');

    actionButton.addEventListener('click', () => {
        updateUrl();
        fetchBlocks();
    });

    updateFromUrl();
}

document.addEventListener('DOMContentLoaded', () => {
    initBlocks();
});
