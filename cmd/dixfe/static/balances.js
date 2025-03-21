// balances.js - Balance-related functionality for DotIDX

// Function to format timestamp as 'DD HH:MM'
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

// Function to build balance graph data
function buildBalanceGraphData(allExtrinsics) {
    const transactionsByDay = {};

    // Process extrinsics to collect time series data
    allExtrinsics.forEach(extrinsic => {
        if (extrinsic.timestamp === 'N/A') {
            return; // Skip entries without valid timestamps
        }

        const date = new Date(extrinsic.timestamp);
        const dayKey = date.toISOString().split('T')[0];
	let amount = extrinsic.totalAmount;

        if (!transactionsByDay[dayKey]) {
            transactionsByDay[dayKey] = {
                date: new Date(dayKey), // Start of the day
                totalAmount: 0,
                deposits: 0,
                withdrawals: 0,
                count: 0
            };
        }

        // Update day totals
        transactionsByDay[dayKey].totalAmount += amount;
	if (amount > 0) {
            transactionsByDay[dayKey].deposits += amount;
	} else {
            transactionsByDay[dayKey].withdrawals += amount;
	}
        transactionsByDay[dayKey].count += 1;
    });

    // Convert the grouped data to an array
    const graphData = Object.values(transactionsByDay);

    // Sort by date (oldest first for cumulative graph)
    return graphData.sort((a, b) => a.date - b.date);
}

// Function to create plotly graph
function createBalanceGraph(graphData, graphDiv) {
    if (graphData.length === 0) {
        graphDiv.innerHTML = '<p class="has-text-centered">No transaction data available for plotting.</p>';
        return;
    }

    // Calculate running balance
    let runningBalance = 0;
    const balanceSeries = graphData.map(item => {
        runningBalance += item.totalAmount;
        return {
            x: item.date,
            y: runningBalance,
            text: `Date: ${item.date.toLocaleDateString()}<br>Balance: ${runningBalance}<br>Day change: ${item.totalAmount}<br>Transactions: ${item.count}`
        };
    });

    // Create data for deposits and withdrawals
    const deposits = graphData.map(item => ({
        x: item.date,
        y: item.deposits,
        text: `Date: ${item.date.toLocaleDateString()}<br>Deposits: +${item.deposits.toFixed(4)}`
    })).filter(item => item.y > 0);

    const withdrawals = graphData.map(item => ({
        x: item.date,
        y: item.withdrawals,
        text: `Date: ${item.date.toLocaleDateString()}<br>Withdrawals: -${item.withdrawals.toFixed(4)}`
    })).filter(item => item.y < 0);

    // Create the plotly data array
    const plotData = [
        {
            type: 'scatter',
            mode: 'lines+markers',
            name: 'Balance',
            x: balanceSeries.map(p => p.x),
            y: balanceSeries.map(p => p.y),
            text: balanceSeries.map(p => p.text),
            line: { color: 'rgb(31, 119, 180)', width: 2 },
            marker: { size: 6 },
            hoverinfo: 'text+x'
        },
        {
            type: 'bar',
            name: 'Deposits',
            x: deposits.map(p => p.x),
            y: deposits.map(p => p.y),
            // text: deposits.map(p => p.text),
            marker: { color: 'rgba(0, 200, 0, 0.7)' },
            hoverinfo: deposits.map(p => '%{x}<br>'+p.text),
            yaxis: 'y2'
        },
        {
            type: 'bar',
            name: 'Withdrawals',
            x: withdrawals.map(p => p.x),
            y: withdrawals.map(p => p.y),
            // text: withdrawals.map(p => p.text),
            marker: { color: 'rgba(200, 0, 0, 0.7)' },
            hoverinfo: withdrawals.map(p => p.x+' '+p.text),
            yaxis: 'y2'
        }
    ];

    // Configure the layout
    const layout = {
        title: {
            text: 'Balance History',
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
            title: 'Balance',
            tickformat: '.2f'
        },
        yaxis2: {
            title: 'Daily Activity',
            titlefont: { color: 'rgb(148, 103, 189)' },
            tickfont: { color: 'rgb(148, 103, 189)' },
            overlaying: 'y',
            side: 'right',
            tickformat: '.2f'
        },
        margin: {
            l: 80,
            r: 80,
            b: 80,
            t: 100,
            pad: 2
        }
    };

    // Create the graph
    Plotly.newPlot(graphDiv, plotData, layout, {responsive: true});
}

// Function to group extrinsics by month
function groupBalancesByMonth(allExtrinsics) {
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

// Function to render extrinsics table
function renderBalancesTable(extrinsicsByMonth) {
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
            let amount = extrinsic.totalAmount;
            let detailsContent = {
                blockId: extrinsic.blockId // Add blockId to details
            };

            html += `<td>${amount}</td>`;

            // Toggle button for details
            const detailsId = `extrinsic-details-${monthKey}-${index}`;
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

function extractBalancesFromBlocks(blocks, address) {
    const balances = [];

    // Go through all blocks and collect extrinsics
    blocks.forEach(block => {
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
		if (extrinsic?.method.pallet) {
		    const palletName = extrinsic.method.pallet
                    if (palletName === 'paraInclusion'
			|| palletName === 'staking'
			|| (palletName === 'utility' && extrinsic.method.method === 'Rewarded')
                       ) {
			return; // Skip paraInclusion extrinsics
                    }

		    let amount = 0.0
		    if (extrinsic.method.pallet === 'balances' && extrinsic.method.method === 'Transfer') {
			amount = parseFloat(extrinsic.data[2])
			if (address === extrinsic.data[0]) {
			    amount = -amount
			}
		    } else if (extrinsic.method.pallet === 'balances' && extrinsic.method.method === 'Deposit') {
			amount = parseFloat(extrinsic.data[1])
		    } else if (extrinsic.method.pallet === 'balances' && extrinsic.method.method === 'Withdraw') {
			amount = -parseFloat(extrinsic.data[1])
		    }

		    amount = amount / 10/1000/1000/1000;

                    balances.push({
			timestamp,
			blockId,
			pallet: palletName,
			method: extrinsic.method,
			totalAmount: amount,
                    });
		}
            });
        });
    });

    return balances;
}

// Function to add toggle listeners for extrinsic details
function addExtrinsicToggleListeners() {
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

// Function to fetch balances
async function fetchBalances() {
    const searchInput = document.getElementById('search-address');
    const address = searchInput.value.trim();
    if (!address) {
        alert('Please enter an address');
        return;
    }

    try {
        // Get filter values
        const count = document.getElementById('balance-count').value;
        const fromDateInput = document.getElementById('balance-from').value;
        const toDateInput = document.getElementById('balance-to').value;

        // Format dates to RFC3339 format
        let fromDate = '';
        let toDate = '';

        if (fromDateInput) {
            // Convert HTML datetime-local input format to RFC3339
            const fromDateTime = new Date(fromDateInput);
            fromDate = fromDateTime.toISOString(); // This gives RFC3339 format
        }

        if (toDateInput) {
            // Convert HTML datetime-local input format to RFC3339
            const toDateTime = new Date(toDateInput);
            toDate = toDateTime.toISOString(); // This gives RFC3339 format
        }

        // Build URL with parameters
        let balancesUrl = `/balances?address=${encodeURIComponent(address)}`;

        if (count) {
            balancesUrl += `&count=${encodeURIComponent(count)}`;
        }

        if (fromDate) {
            balancesUrl += `&from=${encodeURIComponent(fromDate)}`;
        }

        if (toDate) {
            balancesUrl += `&to=${encodeURIComponent(toDate)}`;
        }

        console.log('Fetching balances from URL:', balancesUrl);
        const response = await fetch(balancesUrl);
        if (!response.ok) {
            throw new Error(`HTTP error ${response.status}`);
        }
        const textRaw = await response.text();
        const result = JSON.parse(textRaw);
        console.log('Balances Data:', textRaw); // Debug log

        const resultDiv = document.getElementById('balance-result');
        const dataDiv = document.getElementById('balance-data');
        const graphDiv = document.getElementById('balance-graph');

        resultDiv.classList.remove('is-hidden');

        if (result && Array.isArray(result) && result.length > 0) {
            // Process blocks to extract extrinsics
            const balances = extractBalancesFromBlocks(result, address);

            // Format timestamps for display
            balances.forEach(extrinsic => {
                if (extrinsic.timestamp !== 'N/A') {
                    extrinsic.formattedTime = formatTimestamp(extrinsic.timestamp);
                }
            });

            // Group extrinsics by month
            const balancesByMonth = groupBalancesByMonth(balances);

            // Create graph data
            const graphData = buildBalanceGraphData(balances);
            createBalanceGraph(graphData, graphDiv);

            // Render the table
            dataDiv.innerHTML = renderBalancesTable(balancesByMonth);

            // Add toggle listeners for extrinsic details
            addExtrinsicToggleListeners();
        } else {
            dataDiv.innerHTML = '<p>No balance data found for this address.</p>';
            graphDiv.innerHTML = '';
        }
    } catch (error) {
        console.error('Error fetching balances:', error);
        showError('balances', error.message);
    }
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
    fetchBalances,
    formatTimestamp,
    buildBalanceGraphData,
    createBalanceGraph,
    groupBalancesByMonth,
    renderBalancesTable,
    extractBalancesFromBlocks,
    addExtrinsicToggleListeners
};
