// app.js - Main JavaScript file for DotIDX Dashboard
// This file imports and coordinates all functionality from the module files

// Import functionality from module files
import { fetchBlocks, renderBlockContent, escapeHtml, highlightAddressMatches, escapeRegExp } from './blocks.js';
import { fetchBalances } from './balances.js';
import { fetchStaking } from './staking.js';
import { fetchCompletionRate, fetchMonthlyStats } from './stats.js';

// Wait for DOM to be fully loaded before initializing
document.addEventListener('DOMContentLoaded', () => {
    console.log('DOM loaded, initializing application');
    initApp();
});

// Main application initialization function
function initApp() {
    // Get common elements used throughout the app
    const searchInput = document.getElementById('search-address');
    const actionButton = document.getElementById('action-button');
    const tabsContainer = document.querySelector('.tabs ul');
    const tabContents = document.querySelectorAll('.tab-content');
    let activeTab = '';

    console.log('Elements found:', {
        searchInput: !!searchInput,
        actionButton: !!actionButton,
        tabsContainer: !!tabsContainer,
        tabContents: tabContents.length
    });

    // Process URL parameters
    const urlParams = new URLSearchParams(window.location.search);
    const addressParam = urlParams.get('address');
    const countParam = urlParams.get('count');
    const fromParam = urlParams.get('from');
    const toParam = urlParams.get('to');

    // Populate form fields from URL parameters if present
    if (addressParam) {
        searchInput.value = addressParam;
    }
    if (countParam) {
        document.getElementById('balance-count').value = countParam;
    }
    if (fromParam) {
        // Convert from ISO format to local datetime-local format
        const fromDate = new Date(fromParam);
        document.getElementById('balance-from').value = fromDate.toISOString().slice(0, 16);
    }
    if (toParam) {
        // Convert from ISO format to local datetime-local format
        const toDate = new Date(toParam);
        document.getElementById('balance-to').value = toDate.toISOString().slice(0, 16);
    }

    // Function to fetch completion rate
    async function fetchCompletionRate() {
        try {
            const response = await fetch('/completion_rate');
            if (!response.ok) {
                throw new Error(`HTTP error ${response.status}`);
            }
            const data = await response.json();

            // Process and display completion rate
            const resultDiv = document.getElementById('completion-result');
            const dataDiv = document.getElementById('completion-data');

            resultDiv.classList.remove('is-hidden');

            // Format the completion rate data as a table
            let html = formatJsonToTable(data);
            dataDiv.innerHTML = html;
        } catch (error) {
            console.error('Error fetching completion rate:', error);
            showError('completion', error.message);
        }
    }

    // Function to fetch monthly stats
    async function fetchMonthlyStats() {
        try {
            const response = await fetch('/monthly');
            if (!response.ok) {
                throw new Error(`HTTP error ${response.status}`);
            }
            const data = await response.json();

            const resultDiv = document.getElementById('monthly-result');
            const dataDiv = document.getElementById('monthly-data');

            resultDiv.classList.remove('is-hidden');

            if (Array.isArray(data) && data.length > 0) {
                // Create a table for the monthly stats
                let html = '<table class="table is-fullwidth is-hoverable result-table">';
                html += '<thead><tr><th>Month</th><th>Blocks</th><th>Extrinsics</th><th>Avg Extrinsics per Block</th></tr></thead>';
                html += '<tbody>';

                // Sort by date (newest first)
                data.sort((a, b) => new Date(b.month) - new Date(a.month));

                data.forEach(month => {
                    const monthDate = new Date(month.month);
                    const monthName = monthDate.toLocaleString('default', { year: 'numeric', month: 'long' });
                    const avgExtrinsics = (month.extrinsics / month.blocks).toFixed(2);

                    html += `<tr>
                        <td>${monthName}</td>
                        <td>${month.blocks.toLocaleString()}</td>
                        <td>${month.extrinsics.toLocaleString()}</td>
                        <td>${avgExtrinsics}</td>
                    </tr>`;
                });

                html += '</tbody></table>';
                dataDiv.innerHTML = html;
            } else {
                dataDiv.innerHTML = '<p>No monthly statistics available.</p>';
            }
        } catch (error) {
            console.error('Error fetching monthly stats:', error);
            showError('monthly', error.message);
        }
    }

    // Function to update URL with current parameters based on active tab
    function updateUrl() {
        const address = searchInput.value.trim();
        if (!address) return; // Don't update URL if no address is entered

        const count = document.getElementById('balance-count').value.trim();
        const fromDate = document.getElementById('balance-from').value;
        const toDate = document.getElementById('balance-to').value;

        let newUrl = `?address=${encodeURIComponent(address)}`;

        if (count) {
            newUrl += `&count=${encodeURIComponent(count)}`;
        }

        if (fromDate) {
            const fromDateTime = new Date(fromDate);
            newUrl += `&from=${encodeURIComponent(fromDateTime.toISOString())}`;
        }

        if (toDate) {
            const toDateTime = new Date(toDate);
            newUrl += `&to=${encodeURIComponent(toDateTime.toISOString())}`;
        }

        // Update URL without reloading the page
        window.history.pushState({}, '', newUrl);
    }

    // Function to clear filters and reset to defaults
    function clearFilters() {
        document.getElementById('balance-count').value = '';
        document.getElementById('balance-from').value = '';
        document.getElementById('balance-to').value = '';

        // Also clear the URL parameters
        if (searchInput.value.trim()) {
            // Only keep the address parameter
            window.history.pushState({}, '', `?address=${encodeURIComponent(searchInput.value.trim())}`);
        } else {
            // Clear all parameters
            window.history.pushState({}, '', window.location.pathname);
        }
    }

    // Function to set the active tab
    function setActiveTab(tabId) {
        console.log('Setting active tab to:', tabId);
        activeTab = tabId;

        // Debug DOM elements for each tab to identify issues
        if (tabId === 'balances-tab') {
            const balanceResult = document.getElementById('balance-result');
            const balanceData = document.getElementById('balance-data');
            const balanceGraph = document.getElementById('balance-graph');
            console.log('Balance tab elements:', {
                balanceResult: !!balanceResult,
                balanceData: !!balanceData,
                balanceGraph: !!balanceGraph
            });
        } else if (tabId === 'blocks-tab') {
            const blocksResult = document.getElementById('blocks-result');
            const blocksData = document.getElementById('blocks-data');
            console.log('Blocks tab elements:', {
                blocksResult: !!blocksResult,
                blocksData: !!blocksData
            });
        } else if (tabId === 'stats-tab') {
            const completionResult = document.getElementById('completion-result');
            const monthlyResult = document.getElementById('monthly-result');
            console.log('Stats tab elements:', {
                completionResult: !!completionResult,
                monthlyResult: !!monthlyResult
            });
        }

        // Update the action button behavior based on active tab
        if (tabId === 'balances-tab') {
            actionButton.textContent = 'Search Balances';
            actionButton.onclick = () => {
                console.log('Executing fetchBalances() function');
                fetchBalances();
                updateUrl();
            };
            console.log('Set action button for balances tab');
        } else if (tabId === 'blocks-tab') {
            actionButton.textContent = 'Search Blocks';
            actionButton.onclick = () => {
                fetchBlocks();
                updateUrl();
            };
            console.log('Set action button for blocks tab');
        } else if (tabId === 'stats-tab') {
            actionButton.textContent = 'Refresh Stats';
            actionButton.onclick = () => {
                // Fetch both types of stats when on the stats tab
                fetchStats();
                // These functions check if their elements exist before proceeding
                fetchCompletionRate();
                fetchMonthlyStats();
            };
            console.log('Set action button for stats tab');
        } else if (tabId === 'staking-tab') {
            actionButton.textContent = 'Search Staking';
            actionButton.onclick = () => {
                fetchStaking();
                updateUrl();
            };
            console.log('Set action button for staking tab');
        }

        // Update the active tab styling
        const tabs = document.querySelectorAll('.tabs li');
        console.log('Found', tabs.length, 'tab elements');
        tabs.forEach(tab => {
            const tabDataAttr = tab.getAttribute('data-tab');
            console.log('Tab element:', tabDataAttr, 'comparing with', tabId, 'isMatch:', tabDataAttr === tabId);
            if (tabDataAttr === tabId) {
                tab.classList.add('is-active');
            } else {
                tab.classList.remove('is-active');
            }
        });

        // Update tab content visibility
        const tabContents = document.querySelectorAll('.tab-content');
        console.log('Found', tabContents.length, 'tab content elements');
        tabContents.forEach(content => {
            console.log('Tab content ID:', content.id, 'comparing with', tabId, 'isMatch:', content.id === tabId);
            if (content.id === tabId) {
                console.log('Activating content:', content.id);
                content.classList.add('is-active');
                content.classList.remove('is-hidden');
            } else {
                content.classList.remove('is-active');
                content.classList.add('is-hidden');
            }
        });
    }

    // Initialize the app
    function init() {
        console.log('Initializing application...');

        // Verify tabs and contents are available
        if (!tabsContainer) {
            console.error('No tabs container found!');
        }

        if (tabContents.length === 0) {
            console.error('No tab contents found!');
        }

        // Set initial active tab to match HTML
        const initialActiveTab = document.querySelector('.tabs li.is-active');
        if (initialActiveTab) {
            const initialTabId = initialActiveTab.getAttribute('data-tab');
            console.log('Setting initial active tab to:', initialTabId);
            activeTab = initialTabId;
            setActiveTab(initialTabId);
        } else {
            console.warn('No active tab found in HTML, defaulting to blocks-tab');
            setActiveTab('blocks-tab');
        }

        // Enable the action button to perform a search
        actionButton.addEventListener('click', () => {
            // Get the currently active tab
            const activeTabElement = document.querySelector('.tabs li.is-active');
            if (!activeTabElement) {
                console.error('No active tab found!');
                return;
            }

            const activeTabId = activeTabElement.getAttribute('data-tab');
            console.log('Action button clicked for tab:', activeTabId);

            // Perform action based on active tab
            if (activeTabId === 'balances-tab') {
                fetchBalances();
            } else if (activeTabId === 'blocks-tab') {
                fetchBlocks();
            } else if (activeTabId === 'stats-tab') {
                fetchStats();
                fetchCompletionRate();
                fetchMonthlyStats();
            } else if (activeTabId === 'staking-tab') {
                fetchStaking();
            }

            // Update URL with parameters when applicable
            if (activeTabId !== 'stats-tab') {
                updateUrl();
            }
        });

        // Allow Enter key to trigger search
        searchInput.addEventListener('keyup', (event) => {
            if (event.key === 'Enter') {
                actionButton.click();
            }
        });

        // If address is provided in URL, trigger search automatically
        if (addressParam) {
            actionButton.click();
        }

        // Set up clear filters button if it exists
        const clearFiltersButton = document.getElementById('clear-filters');
        if (clearFiltersButton) {
            clearFiltersButton.addEventListener('click', clearFilters);
        }

        // Initialize filters section if it exists
        const filtersToggle = document.getElementById('filters-toggle');
        if (filtersToggle) {
            filtersToggle.addEventListener('click', function() {
                const filtersSection = document.getElementById('filters-section');
                if (filtersSection) {
                    filtersSection.classList.toggle('is-hidden');
                    this.classList.toggle('is-active');
                }
            });
        }

        // Add event listener for tab clicks
        tabsContainer.addEventListener('click', (event) => {
            // Find the nearest LI element - either the target itself or its parent
            let tabElement = event.target;

            console.log('Tab click event captured on:', tabElement.tagName, tabElement);

            // Handle click on A tag inside LI
            if (tabElement.tagName === 'A') {
                tabElement = tabElement.parentElement; // If clicked on the A tag, get its parent LI
                console.log('Click on A tag, parent is:', tabElement.tagName);
            }

            // Only proceed if we have an LI element
            if (tabElement.tagName === 'LI') {
                const tabId = tabElement.getAttribute('data-tab');
                console.log('Tab element found, data-tab attribute:', tabId);

                if (tabId) {
                    setActiveTab(tabId);
                } else {
                    console.error('Tab element missing data-tab attribute:', tabElement);
                }
            }
        });
    }

    // Helper function to show error messages
    function showError(section, message) {
        const resultDiv = document.getElementById(`${section}-result`);
        const dataDiv = document.getElementById(`${section}-data`);

        if (resultDiv && dataDiv) {
            resultDiv.classList.remove('is-hidden');
            dataDiv.innerHTML = `<div class="notification is-danger">${message}</div>`;
        }
    }

    // Helper function to format JSON as a table
    function formatJsonToTable(data) {
        if (!data || typeof data !== 'object') {
            return '<p>No data available.</p>';
        }

        // Check if it's an array of objects
        if (Array.isArray(data)) {
            if (data.length === 0) {
                return '<p>No data available.</p>';
            }

            // Create table headers from the first object's keys
            const keys = Object.keys(data[0]);
            let html = '<table class="table is-fullwidth is-hoverable result-table">';
            html += '<thead><tr>';

            keys.forEach(key => {
                html += `<th>${key}</th>`;
            });

            html += '</tr></thead><tbody>';

            // Add rows for each object
            data.forEach(item => {
                html += '<tr>';

                keys.forEach(key => {
                    const value = item[key];
                    if (typeof value === 'object' && value !== null) {
                        html += `<td>${JSON.stringify(value)}</td>`;
                    } else {
                        html += `<td>${value}</td>`;
                    }
                });

                html += '</tr>';
            });

            html += '</tbody></table>';
            return html;
        } else {
            // It's a single object, create a key-value table
            let html = '<table class="table is-fullwidth is-hoverable result-table">';
            html += '<thead><tr><th>Key</th><th>Value</th></tr></thead>';
            html += '<tbody>';

            Object.entries(data).forEach(([key, value]) => {
                html += '<tr>';
                html += `<td>${key}</td>`;

                if (typeof value === 'object' && value !== null) {
                    html += `<td>${JSON.stringify(value)}</td>`;
                } else {
                    html += `<td>${value}</td>`;
                }

                html += '</tr>';
            });

            html += '</tbody></table>';
            return html;
        }
    }

    // Call the initialization function
    init();
}
