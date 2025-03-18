// app.js - Vanilla JavaScript for DotIDX Dashboard

document.addEventListener('DOMContentLoaded', () => {
    // Tab switching functionality
    const tabs = document.querySelectorAll('.tabs li');
    const tabContents = document.querySelectorAll('.tab-content');
    let activeTab = 'blocks-tab'; // Default active tab
    
    // Common search input and action button
    const searchInput = document.getElementById('search-address');
    const actionButton = document.getElementById('action-button');
    
    // 1. Check for address parameter in URL and populate the form
    const urlParams = new URLSearchParams(window.location.search);
    const addressParam = urlParams.get('address');
    if (addressParam) {
        searchInput.value = addressParam;
        // Will trigger API call after tabs are set up
    }
    
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            // Remove active class from all tabs and contents
            tabs.forEach(t => t.classList.remove('is-active'));
            tabContents.forEach(c => c.classList.remove('is-active'));
            
            // Add active class to selected tab and content
            tab.classList.add('is-active');
            activeTab = tab.getAttribute('data-tab');
            document.getElementById(activeTab).classList.add('is-active');
            
            // 3. Call the method associated with this tab
            if (searchInput.value.trim()) {
                performActiveTabAction();
            } else {
                // If it's stats tab, we can show stats without an address
                if (activeTab === 'stats-tab') {
                    fetchCompletionRate();
                    fetchMonthlyStats();
                } else {
                    // Hide previous results when switching tabs
                    document.getElementById('blocks-result').classList.add('is-hidden');
                    document.getElementById('balance-result').classList.add('is-hidden');
                }
            }
        });
    });
    
    // Handle action button click based on active tab
    actionButton.addEventListener('click', performActiveTabAction);
    
    // Also trigger action on Enter key in the search input
    searchInput.addEventListener('keyup', (event) => {
        if (event.key === 'Enter') {
            performActiveTabAction();
        }
    });
    
    // Function to perform the action based on active tab
    function performActiveTabAction() {
        switch(activeTab) {
            case 'blocks-tab':
                fetchBlocks();
                break;
            case 'balances-tab':
                fetchBalances();
                break;
            case 'stats-tab':
                // Call both stats functions
                fetchCompletionRate();
                fetchMonthlyStats();
                break;
        }
        
        // Update URL with the address for bookmarking/sharing
        const address = searchInput.value.trim();
        if (address) {
            const url = new URL(window.location.href);
            url.searchParams.set('address', address);
            window.history.replaceState({}, '', url);
        }
    }
    
    // Function to fetch balances
    async function fetchBalances() {
        const address = searchInput.value.trim();
        if (!address) {
            alert('Please enter an address');
            return;
        }

        try {
            const response = await fetch(`/balances?address=${encodeURIComponent(address)}`);
            if (!response.ok) {
                throw new Error(`HTTP error ${response.status}`);
            }
            const textRaw = await response.text();
            const result = JSON.parse(textRaw);
            console.log('Balances Data:', textRaw); // Debug log
            
            const resultDiv = document.getElementById('balance-result');
            const dataDiv = document.getElementById('balance-data');
            
            resultDiv.classList.remove('is-hidden');
            
            // Check if we have data
            if (result && Array.isArray(result) && result.length > 0) {
                // Create a consolidated array of all extrinsics
                const allExtrinsics = [];
                
                // Go through all blocks and collect extrinsics
                result.forEach(block => {
                    if (!block.extrinsics || typeof block.extrinsics !== 'object') {
                        return; // Skip blocks without extrinsics
                    }
                    
                    const timestamp = block.timestamp || 'N/A';
                    const blockId = block.number || 'N/A';
                    
                    // Go through each extrinsic type in the block
                    Object.entries(block.extrinsics).forEach(([palletName, extrinsicArray]) => {
                        if (!Array.isArray(extrinsicArray) || extrinsicArray.length === 0) {
                            return; // Skip empty arrays
                        }
                        
                        // Add each extrinsic to the consolidated array
                        extrinsicArray.forEach(extrinsic => {
                            allExtrinsics.push({
                                timestamp,
                                blockId,
                                pallet: palletName,
                                method: extrinsic.method || 'N/A',
                                data: extrinsic.data || [],
                                rawExtrinsic: extrinsic // Keep the full extrinsic for reference
                            });
                        });
                    });
                });
                
                // No extrinsics found across any blocks
                if (allExtrinsics.length === 0) {
                    dataDiv.innerHTML = '<p>No extrinsics found for this address.</p>';
                    return;
                }
                
                // Start building the table
                let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
                html += '<thead><tr><th>Timestamp</th><th>Block ID</th><th>Pallet</th><th>Method</th><th>Amount (DOT)</th><th>Actions</th></tr></thead>';
                html += '<tbody>';
                
                // Add each extrinsic as a row
                allExtrinsics.forEach((extrinsic, index) => {
                    // Main row
                    html += '<tr>';
                    html += `<td>${extrinsic.timestamp}</td>`;
                    html += `<td>${extrinsic.blockId}</td>`;
                    html += `<td>${extrinsic.pallet}</td>`;
                    html += `<td>${extrinsic.method.pallet}/${extrinsic.method.method}</td>`;
                    
                    // Amount handling
                    let amount = '';
                    let detailsContent = {};
                    
                    // Handle special methods: withdraw and deposit
                    if (extrinsic.method.method === 'Withdraw' || 
                        extrinsic.method.method === 'Deposit' || 
                        extrinsic.method.method === 'Rewarded'
                    ) {
                        // Verify data is an array with at least 2 elements
                        if (Array.isArray(extrinsic.data) && extrinsic.data.length >= 2) {
                            // The second element (index 1) is typically the amount
                            const amountValue = extrinsic.data[1]/10000000000;
                            const sign = extrinsic.method.method === 'Withdraw' ? '-' : '+';
                            amount = `${sign}${amountValue}`;
                            
                            // Add relevant data to details
                            if (extrinsic.data.length > 0) {
                                detailsContent = {
                                    address: extrinsic.data[0],
                                    amount: amountValue
                                };
                            }
                        } else {
                            amount = 'N/A';
                            detailsContent = { data: extrinsic.data };
                        }
                    } else {
                        // For other methods, show the raw data
                        amount = 'N/A';
                        detailsContent = { data: extrinsic.data };
                        
                        // Keep all other fields from the extrinsic for reference
                        Object.entries(extrinsic.rawExtrinsic).forEach(([key, value]) => {
                            if (!['method', 'data'].includes(key)) {
                                detailsContent[key] = value;
                            }
                        });
                    }
                    
                    html += `<td>${amount}</td>`;
                    
                    // Toggle button for details
                    const detailsId = `extrinsic-details-${index}`;
                    html += `<td><button class="button is-small toggle-details" data-target="${detailsId}"><i class="fas fa-chevron-right"></i></button></td>`;
                    html += '</tr>';
                    
                    // Details row (hidden by default)
                    html += `<tr id="${detailsId}" class="details-row" style="display: none;">`;
                    html += `<td colspan="6"><pre class="extrinsic-details">${JSON.stringify(detailsContent, null, 2)}</pre></td>`;
                    html += '</tr>';
                });
                
                html += '</tbody></table>';
                dataDiv.innerHTML = html;
                
                // Add some styling for the JSON details
                const style = document.createElement('style');
                style.textContent = `
                    .extrinsic-details {
                        max-height: 150px;
                        overflow-y: auto;
                        font-size: 0.8rem;
                        background-color: #f8f8f8;
                        padding: 0.5rem;
                        border-radius: 4px;
                        margin: 0;
                    }
                    .details-row td {
                        padding: 0 !important;
                    }
                    .details-row pre {
                        margin: 0.75rem;
                    }
                    .toggle-details i {
                        transition: transform 0.2s;
                    }
                    .toggle-details.is-active i {
                        transform: rotate(90deg);
                    }
                `;
                document.head.appendChild(style);
                
                // Add event listeners for the toggle buttons
                document.querySelectorAll('.toggle-details').forEach(button => {
                    button.addEventListener('click', function() {
                        const targetId = this.getAttribute('data-target');
                        const targetRow = document.getElementById(targetId);
                        
                        if (targetRow.style.display === 'none') {
                            targetRow.style.display = 'table-row';
                            this.classList.add('is-active');
                        } else {
                            targetRow.style.display = 'none';
                            this.classList.remove('is-active');
                        }
                    });
                });
            } else {
                dataDiv.innerHTML = '<p>No balance information found for this address.</p>';
            }
        } catch (error) {
            console.error('Error fetching balance:', error);
            showError('balances', error.message);
        }
    }

    // Function to fetch blocks
    async function fetchBlocks() {
        const address = searchInput.value.trim();
        if (!address) {
            alert('Please enter an address');
            return;
        }

        try {
            const response = await fetch(`/address2blocks?address=${encodeURIComponent(address)}`);
            if (!response.ok) {
                throw new Error(`HTTP error ${response.status}`);
            }
            const data = await response.json();
            
            const resultDiv = document.getElementById('blocks-result');
            const dataDiv = document.getElementById('blocks-data');
            
            resultDiv.classList.remove('is-hidden');
            
            if (data && Array.isArray(data) && data.length > 0) {
                let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
                html += '<thead><tr><th>Block Number</th><th>Hash</th></tr></thead>';
                html += '<tbody>';
                
                data.forEach(item => {
                    html += `<tr><td>${item["block_number"] || 'N/A'}</td><td class="is-family-monospace">${item["hash"] || 'N/A'}</td></tr>`;
                });
                
                html += '</tbody></table>';
                dataDiv.innerHTML = html;
            } else {
                dataDiv.innerHTML = '<p>No blocks found for this address.</p>';
            }
        } catch (error) {
            console.error('Error fetching blocks:', error);
            showError('blocks', error.message);
        }
    }

    // Function to fetch completion rate
    async function fetchCompletionRate() {
        try {
            const response = await fetch('/stats/completion_rate');
            if (!response.ok) {
                throw new Error(`HTTP error ${response.status}`);
            }
            const data = await response.json();
            console.log('Completion Rate Data:', data); // Debug log
            
            const resultDiv = document.getElementById('completion-rate-result');
            const dataDiv = document.getElementById('completion-rate-data');
            
            resultDiv.classList.remove('is-hidden');
            
            if (data && typeof data === 'object') {
                // Custom formatting for completion rate with 4 columns
                let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
                html += '<thead><tr><th>Relay Chain</th><th>Chain</th><th>Completion %</th><th>Head ID</th></tr></thead>';
                html += '<tbody><tr>';
                html += `<td>${data["RelayChain"] || 'N/A'}</td>`;
                html += `<td>${data["Chain"] || 'N/A'}</td>`;
                
                // Handle completion rate properly
                let completionPercent = data["percent_completion"];
                if (completionPercent !== undefined) {
                    if (completionPercent <= 1) {
                        // If it's a decimal (e.g., 0.75), convert to percentage
                        completionPercent = (completionPercent * 100).toFixed(2) + '%';
                    } else if (typeof completionPercent === 'number') {
                        // If it's already a percentage but without the % sign
                        completionPercent = completionPercent.toFixed(2) + '%';
                    }
                } else {
                    completionPercent = 'N/A';
                }
                
                html += `<td>${completionPercent}</td>`;
                html += `<td>${data["head_id"] || 'N/A'}</td>`;
                html += '</tr></tbody></table>';
                dataDiv.innerHTML = html;
            } else {
                dataDiv.innerHTML = '<p>No completion rate data available or invalid format.</p>';
            }
        } catch (error) {
            console.error('Error fetching completion rate:', error);
            showError('completion-rate', error.message);
        }
    }

    // Function to fetch monthly stats
    async function fetchMonthlyStats() {
        try {
            const response = await fetch('/stats/per_month');
            if (!response.ok) {
                throw new Error(`HTTP error ${response.status}`);
            }
            const textRaw = await response.text();
            const text = JSON.parse(textRaw);
            console.log('Monthly Stats Data:', textRaw); // Debug log
            
            const resultDiv = document.getElementById('monthly-stats-result');
            const dataDiv = document.getElementById('monthly-stats-data');
            
            resultDiv.classList.remove('is-hidden');
            
            // Always create a 4-column table regardless of data structure
            let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
            html += '<thead><tr><th>Date</th><th>Count</th><th>Min Block</th><th>Max Block</th></tr></thead>';
            html += '<tbody>';
            let data = text.data;
            if (data && Array.isArray(data) && data.length > 0) {
                // Handle array of objects
                data.forEach(item => {
                    // Use bracket notation for accessing properties to avoid any issues
                    const date = item["date"] || 'N/A'; 
                    const count = item["count"] || 0;
                    const minBlock = item["min_block"] || 'N/A';
                    const maxBlock = item["max_block"] || 'N/A';
                    
                    html += '<tr>';
                    html += `<td>${date}</td>`;
                    html += `<td>${count}</td>`;
                    html += `<td>${minBlock}</td>`;
                    html += `<td>${maxBlock}</td>`;
                    html += '</tr>';
                });
            } else if (data && typeof data === 'object' && !Array.isArray(data)) {
                // Handle a single object (not in an array) - create a single row
                const date = data["date"] || 'N/A';
                const count = data["count"] || 0;
                const minBlock = data["min_block"] || 'N/A';
                const maxBlock = data["max_block"] || 'N/A';
                
                html += '<tr>';
                html += `<td>${date}</td>`;
                html += `<td>${count}</td>`;
                html += `<td>${minBlock}</td>`;
                html += `<td>${maxBlock}</td>`;
                html += '</tr>';
            } else {
                // No valid data, show empty row
                html += '<tr><td colspan="4">No monthly statistics available</td></tr>';
            }
            
            html += '</tbody></table>';
            dataDiv.innerHTML = html;
        } catch (error) {
            console.error('Error fetching monthly stats:', error);
            showError('monthly-stats', error.message);
        }
    }
    
    // Add a selector for stats type when on the stats tab
    const completionRateTitle = document.querySelector('#stats-tab h3:first-of-type');
    completionRateTitle.addEventListener('click', () => {
        fetchCompletionRate();
    });
    
    const monthlyStatsTitle = document.querySelector('#stats-tab h3:last-of-type');
    monthlyStatsTitle.addEventListener('click', () => {
        fetchMonthlyStats();
    });

    // Add mobile menu toggler
    const navbarBurgers = Array.prototype.slice.call(document.querySelectorAll('.navbar-burger'), 0);
    if (navbarBurgers.length > 0) {
        navbarBurgers.forEach(el => {
            el.addEventListener('click', () => {
                const target = document.getElementById(el.dataset.target);
                el.classList.toggle('is-active');
                target.classList.toggle('is-active');
            });
        });
    }
    
    // Helper function to show error messages
    function showError(section, message) {
        const dataDiv = document.getElementById(`${section}-data`);
        if (dataDiv) {
            dataDiv.innerHTML = `<div class="notification is-danger">
                <p><strong>Error:</strong> ${message || 'Something went wrong. Please try again.'}</p>
            </div>`;
        }
    }
    
    // Helper function to format JSON as a table
    function formatJsonToTable(data) {
        if (typeof data !== 'object' || data === null) {
            return JSON.stringify(data);
        }
        
        // Handle arrays of objects
        if (Array.isArray(data) && data.length > 0 && typeof data[0] === 'object') {
            const headers = Object.keys(data[0]);
            let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
            
            // Create headers
            html += '<thead><tr>';
            headers.forEach(header => {
                html += `<th>${header}</th>`;
            });
            html += '</tr></thead>';
            
            // Create rows
            html += '<tbody>';
            data.forEach(item => {
                html += '<tr>';
                headers.forEach(header => {
                    const value = item[header];
                    if (typeof value === 'object' && value !== null) {
                        html += `<td>${JSON.stringify(value)}</td>`;
                    } else {
                        html += `<td>${value !== undefined ? value : 'N/A'}</td>`;
                    }
                });
                html += '</tr>';
            });
            html += '</tbody></table>';
            return html;
        }
        
        // Handle simple objects
        let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
        html += '<thead><tr><th>Key</th><th>Value</th></tr></thead>';
        html += '<tbody>';
        
        Object.entries(data).forEach(([key, value]) => {
            html += '<tr>';
            html += `<td>${key}</td>`;
            
            if (typeof value === 'object' && value !== null) {
                html += `<td>${JSON.stringify(value)}</td>`;
            } else {
                html += `<td>${value !== undefined ? value : 'N/A'}</td>`;
            }
            html += '</tr>';
        });
        
        html += '</tbody></table>';
        return html;
    }
    
    // If address parameter was present, trigger appropriate action
    if (addressParam) {
        // Wait for everything to be set up
        setTimeout(performActiveTabAction, 100);
    }
});
