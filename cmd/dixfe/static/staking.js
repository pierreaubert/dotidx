// staking.js - Staking-related functionality for DotIDX

// Function to format timestamp as 'DD HH:MM' (reused from balances.js)
function formatTimestamp(timestamp) {
    if (timestamp === 'N/A') return timestamp;
    
    try {
        const date = new Date(timestamp);
        const day = String(date.getDate()).padStart(2, '0');
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        return `${day} ${hours}:${minutes}`;
    } catch (e) {
        return timestamp;
    }
}

// Function to extract extrinsics from blocks for staking
function extractStakingExtrinsicsFromBlocks(blocks) {
    const allExtrinsics = [];
    
    // Go through all blocks and collect staking extrinsics
    blocks.forEach(block => {
        if (!block.extrinsics || typeof block.extrinsics !== 'object') {
            return; // Skip blocks without extrinsics
        }
        
        const timestamp = block.timestamp || 'N/A';
        const blockId = block.number || 'N/A';
        
        // Go through each pallet in the block
        Object.entries(block.extrinsics).forEach(([palletName, extrinsicArray]) => {
            if (!Array.isArray(extrinsicArray) || extrinsicArray.length === 0) {
                return; // Skip empty arrays
            }
            
            // Add each extrinsic related to staking
            extrinsicArray.forEach(extrinsic => {
                // Check if this is a staking extrinsic or utility with staking method
                if (palletName === 'staking' || 
                    (palletName === 'utility' && 
                     extrinsic.method && 
                     extrinsic.method.pallet === 'staking')) {
                    
                    allExtrinsics.push({
                        timestamp,
                        blockId,
                        pallet: palletName,
                        method: extrinsic.method || 'N/A',
                        data: extrinsic.data || [],
                        rawExtrinsic: extrinsic // Keep the full extrinsic for reference
                    });
                }
            });
        });
    });
    
    return allExtrinsics;
}

// Function to build staking graph data
function buildStakingGraphData(allExtrinsics) {
    const transactionsByDay = {};
    
    // Process extrinsics to collect time series data
    allExtrinsics.forEach(extrinsic => {
        if (extrinsic.timestamp === 'N/A') {
            return; // Skip entries without valid timestamps
        }
        
        try {
            const date = new Date(extrinsic.timestamp);
            // Create a day key (YYYY-MM-DD) to group by day
            const dayKey = date.toISOString().split('T')[0];
            
            // Initialize the day entry if it doesn't exist
            if (!transactionsByDay[dayKey]) {
                transactionsByDay[dayKey] = {
                    date: new Date(dayKey), // Start of the day
                    totalAmount: 0,
                    bonded: 0,
                    unbonded: 0,
                    rewards: 0,
                    count: 0
                };
            }
            
            // Process different staking methods
            let amount = 0;
            let type = 'other';
            
            // Bond
            if ((extrinsic.method.method === 'Bond' || 
                 extrinsic.method.method === 'BondExtra') && 
                Array.isArray(extrinsic.data) && extrinsic.data.length > 0) {
                type = 'bonded';
                amount = parseFloat(extrinsic.data[0])/10000000000;
                transactionsByDay[dayKey].bonded += amount;
                transactionsByDay[dayKey].totalAmount += amount;
            }
            // Unbond
            else if (extrinsic.method.method === 'Unbond' && 
                     Array.isArray(extrinsic.data) && extrinsic.data.length > 0) {
                type = 'unbonded';
                amount = -parseFloat(extrinsic.data[0])/10000000000;
                transactionsByDay[dayKey].unbonded += Math.abs(amount);
                transactionsByDay[dayKey].totalAmount += amount;
            }
            // Rewards
            else if (extrinsic.method.method === 'Rewarded' && 
                     Array.isArray(extrinsic.data) && extrinsic.data.length > 1) {
                type = 'rewards';
                amount = parseFloat(extrinsic.data[1])/10000000000;
                transactionsByDay[dayKey].rewards += amount;
                transactionsByDay[dayKey].totalAmount += amount;
            }
            
            // Update count regardless of type
            transactionsByDay[dayKey].count += 1;
        } catch (e) {
            console.error('Error processing staking extrinsic:', e);
        }
    });
    
    // Convert the grouped data to an array
    const graphData = Object.values(transactionsByDay);
    
    // Sort by date (oldest first for cumulative graph)
    return graphData.sort((a, b) => a.date - b.date);
}

// Function to create plotly graph for staking data
function createStakingGraph(graphData, graphDiv) {
    if (graphData.length === 0) {
        graphDiv.innerHTML = '<p class="has-text-centered">No staking data available for plotting.</p>';
        return;
    }
    
    // Calculate running balance
    let runningBalance = 0;
    const balanceSeries = graphData.map(item => {
        runningBalance += item.totalAmount;
        return {
            x: item.date,
            y: runningBalance,
            text: `Date: ${item.date.toLocaleDateString()}<br>Total Staked: ${runningBalance.toFixed(4)}<br>Day change: ${item.totalAmount.toFixed(4)}<br>Transactions: ${item.count}`
        };
    });
    
    // Create data for bonds, unbonds, and rewards
    const bonded = graphData.map(item => ({
        x: item.date,
        y: item.bonded,
        text: `Date: ${item.date.toLocaleDateString()}<br>Bonded: +${item.bonded.toFixed(4)}`
    })).filter(item => item.y > 0);
    
    const unbonded = graphData.map(item => ({
        x: item.date,
        y: item.unbonded,
        text: `Date: ${item.date.toLocaleDateString()}<br>Unbonded: -${item.unbonded.toFixed(4)}`
    })).filter(item => item.y > 0);
    
    const rewards = graphData.map(item => ({
        x: item.date,
        y: item.rewards,
        text: `Date: ${item.date.toLocaleDateString()}<br>Rewards: +${item.rewards.toFixed(4)}`
    })).filter(item => item.y > 0);
    
    // Create the plotly data array
    const plotData = [
        {
            type: 'scatter',
            mode: 'lines+markers',
            name: 'Total Staked',
            x: balanceSeries.map(p => p.x),
            y: balanceSeries.map(p => p.y),
            text: balanceSeries.map(p => p.text),
            line: { color: 'rgb(31, 119, 180)', width: 2 },
            marker: { size: 6 },
            hoverinfo: 'text+x'
        },
        {
            type: 'bar',
            name: 'Bonded',
            x: bonded.map(p => p.x),
            y: bonded.map(p => p.y),
            text: bonded.map(p => p.text),
            marker: { color: 'rgba(0, 200, 0, 0.7)' },
            hoverinfo: 'text+x',
            yaxis: 'y2'
        },
        {
            type: 'bar',
            name: 'Unbonded',
            x: unbonded.map(p => p.x),
            y: unbonded.map(p => p.y),
            text: unbonded.map(p => p.text),
            marker: { color: 'rgba(200, 0, 0, 0.7)' },
            hoverinfo: 'text+x',
            yaxis: 'y2'
        },
        {
            type: 'bar',
            name: 'Rewards',
            x: rewards.map(p => p.x),
            y: rewards.map(p => p.y),
            text: rewards.map(p => p.text),
            marker: { color: 'rgba(255, 165, 0, 0.7)' },
            hoverinfo: 'text+x',
            yaxis: 'y2'
        }
    ];
    
    // Configure the layout
    const layout = {
        title: {
            text: 'Staking History',
            font: {
                size: 24
            },
            xanchor: 'left',
            x: 0
        },
        showlegend: true,
        legend: {
            orientation: 'h',
            y: -0.2
        },
        hovermode: 'closest',
        xaxis: {
            title: 'Date'
        },
        yaxis: {
            title: 'Total Staked',
            tickformat: '.4f'
        },
        yaxis2: {
            title: 'Daily Activity',
            titlefont: { color: 'rgb(148, 103, 189)' },
            tickfont: { color: 'rgb(148, 103, 189)' },
            overlaying: 'y',
            side: 'right',
            tickformat: '.4f'
        },
        margin: {
            l: 50,
            r: 50,
            b: 100,
            t: 100,
            pad: 4
        }
    };
    
    // Create the graph
    Plotly.newPlot(graphDiv, plotData, layout, {responsive: true});
}

// Function to group staking extrinsics by month
function groupStakingExtrinsicsByMonth(allExtrinsics) {
    const extrinsicsByMonth = {};
    
    allExtrinsics.forEach(extrinsic => {
        if (extrinsic.timestamp !== 'N/A') {
            try {
                // Parse the timestamp
                const date = new Date(extrinsic.timestamp);
                
                // Create a month key in YYYY-MM format
                const monthKey = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
                
                // Initialize the month array if it doesn't exist
                if (!extrinsicsByMonth[monthKey]) {
                    extrinsicsByMonth[monthKey] = [];
                }
                
                // Add the extrinsic to the month
                extrinsicsByMonth[monthKey].push(extrinsic);
            } catch (e) {
                // Handle invalid timestamps
                if (!extrinsicsByMonth['Unknown']) {
                    extrinsicsByMonth['Unknown'] = [];
                }
                extrinsicsByMonth['Unknown'].push(extrinsic);
            }
        } else {
            // Handle invalid timestamps
            if (!extrinsicsByMonth['Unknown']) {
                extrinsicsByMonth['Unknown'] = [];
            }
            extrinsicsByMonth['Unknown'].push(extrinsic);
        }
    });
    
    return extrinsicsByMonth;
}

// Function to render staking extrinsics table
function renderStakingExtrinsicsTable(extrinsicsByMonth) {
    // Start building the table
    let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
    html += '<thead><tr><th>Timestamp</th><th>Pallet</th><th>Method</th><th>Amount (DOT)</th><th>Details</th></tr></thead>';
    html += '<tbody>';
    
    // Sort month keys in descending order (newest first)
    const sortedMonths = Object.keys(extrinsicsByMonth).sort((a, b) => {
        // Handle 'Unknown' specially
        if (a === 'Unknown') return 1;
        if (b === 'Unknown') return -1;
        return b.localeCompare(a); // Descending order
    });
    
    // Process each month group
    sortedMonths.forEach(monthKey => {
        // Add month header row
        const monthName = monthKey === 'Unknown' ? 'Unknown Date' : 
            new Date(`${monthKey}-01`).toLocaleString('default', { year: 'numeric', month: 'long' });
        
        html += `<tr class="month-header"><td colspan="5"><strong>${monthName}</strong></td></tr>`;
        
        // Add extrinsics for this month
        extrinsicsByMonth[monthKey].forEach((extrinsic, index) => {
            // Main row
            html += '<tr>';
            html += `<td>${extrinsic.formattedTime || extrinsic.timestamp}</td>`;
            html += `<td>${extrinsic.pallet}</td>`;
            html += `<td>${extrinsic.method.pallet}/${extrinsic.method.method}</td>`;
            
            // Amount handling
            let amount = 'N/A';
            let detailsContent = {
                blockId: extrinsic.blockId // Add blockId to details
            };
            
            // Handle specific staking methods
            if (extrinsic.method.method === 'Bond' || 
                extrinsic.method.method === 'BondExtra') {
                if (Array.isArray(extrinsic.data) && extrinsic.data.length > 0) {
                    const amountValue = extrinsic.data[0]/10000000000;
                    amount = `+${amountValue}`;
                    detailsContent.amount = amountValue;
                }
            } else if (extrinsic.method.method === 'Unbond') {
                if (Array.isArray(extrinsic.data) && extrinsic.data.length > 0) {
                    const amountValue = extrinsic.data[0]/10000000000;
                    amount = `-${amountValue}`;
                    detailsContent.amount = -amountValue;
                }
            } else if (extrinsic.method.method === 'Rewarded') {
                if (Array.isArray(extrinsic.data) && extrinsic.data.length > 1) {
                    const amountValue = extrinsic.data[1]/10000000000;
                    amount = `+${amountValue}`;
                    detailsContent.amount = amountValue;
                    detailsContent.account = extrinsic.data[0];
                }
            }
            
            // Add raw data to details
            detailsContent.data = extrinsic.data;
            
            // Keep all other fields from the extrinsic for reference
            Object.entries(extrinsic.rawExtrinsic).forEach(([key, value]) => {
                if (!['method', 'data'].includes(key)) {
                    detailsContent[key] = value;
                }
            });
            
            html += `<td>${amount}</td>`;
            
            // Toggle button for details
            const detailsId = `staking-details-${monthKey}-${index}`;
            html += `<td><button class="button is-small toggle-details" data-target="${detailsId}"><i class="fas fa-chevron-right"></i></button></td>`;
            html += '</tr>';
            
            // Details row (hidden by default)
            html += `<tr id="${detailsId}" class="details-row" style="display: none;">`;
            html += `<td colspan="5"><pre class="extrinsic-details">${JSON.stringify(detailsContent, null, 2)}</pre></td>`;
            html += '</tr>';
        });
    });
    
    html += '</tbody></table>';
    return html;
}

// Function to fetch staking data
async function fetchStaking() {
    const searchInput = document.getElementById('search-address');
    const address = searchInput.value.trim();
    if (!address) {
        alert('Please enter an address');
        return;
    }

    try {
        // Get filter values - reuse the same filters as balances
        const count = document.getElementById('balance-count').value;
        const fromDateInput = document.getElementById('balance-from').value;
        const toDateInput = document.getElementById('balance-to').value;
        
        // Format dates to RFC3339 format
        let fromDate = '';
        let toDate = '';
        
        if (fromDateInput) {
            const fromDateTime = new Date(fromDateInput);
            fromDate = fromDateTime.toISOString();
        }
        
        if (toDateInput) {
            const toDateTime = new Date(toDateInput);
            toDate = toDateTime.toISOString();
        }
        
        // For now, reuse the address2blocks endpoint to get staking data
        // We'll filter for staking-related extrinsics on the client side
        let url = `/address2blocks?address=${encodeURIComponent(address)}`;
        
        if (count) {
            url += `&count=${encodeURIComponent(count)}`;
        }
        
        if (fromDate) {
            url += `&from=${encodeURIComponent(fromDate)}`;
        }
        
        if (toDate) {
            url += `&to=${encodeURIComponent(toDate)}`;
        }
        
        console.log('Fetching staking data from URL:', url);
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error ${response.status}`);
        }
        const data = await response.json();
        
        const resultDiv = document.getElementById('staking-result');
        const dataDiv = document.getElementById('staking-data');
        const graphDiv = document.getElementById('staking-graph');
        
        resultDiv.classList.remove('is-hidden');
        
        if (data && Array.isArray(data) && data.length > 0) {
            // Process blocks to extract staking extrinsics
            const stakingExtrinsics = extractStakingExtrinsicsFromBlocks(data);
            
            // Format timestamps for display
            stakingExtrinsics.forEach(extrinsic => {
                if (extrinsic.timestamp !== 'N/A') {
                    extrinsic.formattedTime = formatTimestamp(extrinsic.timestamp);
                }
            });
            
            if (stakingExtrinsics.length > 0) {
                // Group extrinsics by month
                const extrinsicsByMonth = groupStakingExtrinsicsByMonth(stakingExtrinsics);
                
                // Create graph data
                const graphData = buildStakingGraphData(stakingExtrinsics);
                
                // Create the graph
                createStakingGraph(graphData, graphDiv);
                
                // Render the table
                dataDiv.innerHTML = renderStakingExtrinsicsTable(extrinsicsByMonth);
                
                // Add toggle listeners for extrinsic details
                addStakingToggleListeners();
            } else {
                dataDiv.innerHTML = '<p>No staking data found for this address.</p>';
                graphDiv.innerHTML = '';
            }
        } else {
            dataDiv.innerHTML = '<p>No blocks found for this address.</p>';
            graphDiv.innerHTML = '';
        }
    } catch (error) {
        console.error('Error fetching staking data:', error);
        showError('staking', error.message);
    }
}

// Function to add toggle listeners for staking extrinsic details
function addStakingToggleListeners() {
    document.querySelectorAll('.toggle-details').forEach(button => {
        button.addEventListener('click', function() {
            const targetId = this.getAttribute('data-target');
            const targetRow = document.getElementById(targetId);
            if (targetRow) {
                const isVisible = targetRow.style.display !== 'none';
                targetRow.style.display = isVisible ? 'none' : 'table-row';
                this.classList.toggle('is-active');
            }
        });
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

// Export functions
export {
    fetchStaking,
    formatTimestamp,
    buildStakingGraphData,
    createStakingGraph,
    groupStakingExtrinsicsByMonth,
    renderStakingExtrinsicsTable,
    extractStakingExtrinsicsFromBlocks,
    addStakingToggleListeners
};
