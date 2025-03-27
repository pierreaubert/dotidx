// import Plotly from "plotly.js-dist-min";
import { showError, formatTimestamp } from './misc.js';
import { getAccountAt } from './accounts.js';
import { initAddresses } from './assets.js';

// Function to build balance graph data
function buildBalanceGraphData(balances) {
    const transactionsByDay = {};

    // Process extrinsics to collect time series data
    balances.forEach((extrinsic) => {
        const date = new Date(extrinsic.timestamp);
        const dayKey = date.toISOString().split('T')[0];
        let amount = extrinsic.amount;
        let totalAmount = extrinsic.totalAmount;

        if (!transactionsByDay[dayKey]) {
            transactionsByDay[dayKey] = {
                date: new Date(dayKey), // Start of the day
                amount: 0,
                totalAmount: 0,
                deposits: 0,
                withdrawals: 0,
                count: 0,
            };
        }

        transactionsByDay[dayKey].amount += amount;
        transactionsByDay[dayKey].totalAmount = totalAmount;
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
function createBalanceGraph(graphData, graphDiv, address) {
    if (graphData.length === 0) {
        graphDiv.innerHTML = '<p class="has-text-centered">No transaction data available for plotting.</p>';
        return;
    }

    // Calculate running balance
    let runningBalance = graphData[0].totalAmount;
    const balanceSeries = graphData.map((item) => {
        runningBalance += item.amount;
        return {
            x: item.date,
            y: runningBalance,
        };
    });

    // Create data for deposits and withdrawals
    const deposits = graphData
        .map((item) => ({
            x: item.date,
            y: item.deposits,
            text: `Date: ${item.date.toLocaleDateString()}<br>Deposits: +${item.deposits}`,
        }))
        .filter((item) => item.y > 0);

    const withdrawals = graphData
        .map((item) => ({
            x: item.date,
            y: item.withdrawals,
            text: `Date: ${item.date.toLocaleDateString()}<br>Withdrawals: -${item.withdrawals}`,
        }))
        .filter((item) => item.y < 0);

    // Create the plotly data array
    const plotData = [
        {
            type: 'scatter',
            mode: 'lines+markers',
            name: 'Balance ' + address,
            x: balanceSeries.map((p) => p.x),
            y: balanceSeries.map((p) => p.y),
            text: balanceSeries.map((p) => p.text),
            line: { color: 'rgb(31, 119, 180)', width: 2 },
            marker: { size: 6 },
            hoverinfo: 'text+x',
        },
        {
            type: 'bar',
            name: 'Deposits',
            x: deposits.map((p) => p.x),
            y: deposits.map((p) => p.y),
            // text: deposits.map(p => p.text),
            marker: { color: 'rgba(0, 200, 0, 0.7)' },
            hoverinfo: deposits.map((p) => '%{x}<br>' + p.text),
            yaxis: 'y2',
        },
        {
            type: 'bar',
            name: 'Withdrawals',
            x: withdrawals.map((p) => p.x),
            y: withdrawals.map((p) => p.y),
            // text: withdrawals.map(p => p.text),
            marker: { color: 'rgba(200, 0, 0, 0.7)' },
            hoverinfo: withdrawals.map((p) => p.x + ' ' + p.text),
            yaxis: 'y2',
        },
    ];

    // Configure the layout
    const layout = {
        showlegend: true,
        legend: {
            orientation: 'h',
            y: -0.2,
        },
        hovermode: 'closest',
        xaxis: {
            title: 'Date',
        },
        yaxis: {
            title: 'Balance',
            tickformat: '.0f',
        },
        yaxis2: {
            title: 'Daily Activity',
            titlefont: { color: 'rgb(148, 103, 189)' },
            tickfont: { color: 'rgb(148, 103, 189)' },
            overlaying: 'y',
            side: 'right',
            tickformat: '.0f',
        },
        margin: {
            l: 80,
            r: 80,
            b: 80,
            t: 40,
            pad: 2,
        },
    };

    // Create the graph
    Plotly.newPlot(graphDiv, plotData, layout, { responsive: true });
}

// Function to group extrinsics by month
function groupBalancesByMonth(allExtrinsics) {
    const extrinsicsByMonth = {};

    allExtrinsics.forEach((extrinsic) => {
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
                console.error('invalid timestamp', e);
                if (!extrinsicsByMonth['Unknown']) {
                    extrinsicsByMonth['Unknown'] = [];
                }
                extrinsicsByMonth['Unknown'].push(extrinsic);
            }
        } else {
            if (!extrinsicsByMonth['Unknown']) {
                extrinsicsByMonth['Unknown'] = [];
            }
            extrinsicsByMonth['Unknown'].push(extrinsic);
        }
    });

    return extrinsicsByMonth;
}

function renderBalancesTable(extrinsicsByMonth) {
    let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
    html += `
	 <thead>
           <tr>
             <th>Timestamp</th>
             <th>Method</th>
             <th class="has-text-right">Amount (DOT)</th>
             <th class="has-text-right">Balance (DOT)</th>
             <th>Details</th></tr>
         </thead>
`;
    html += '<tbody>';

    // Sort month keys in descending order (newest first)
    const sortedMonths = Object.keys(extrinsicsByMonth).sort((a, b) => {
        // Handle 'Unknown' specially
        if (a === 'Unknown') return 1;
        if (b === 'Unknown') return -1;
        return b.localeCompare(a); // Descending order
    });

    // Process each month group
    sortedMonths.forEach((monthKey) => {
        // Add month header row
        const monthName =
            monthKey === 'Unknown'
                ? 'Unknown Date'
                : new Date(`${monthKey}-01`).toLocaleString('default', {
                      year: 'numeric',
                      month: 'long',
                  });

        html += `<tr class="month-header"><td colspan="4"><strong>${monthName}</strong></td></tr>`;

        // Add extrinsics for this month
        extrinsicsByMonth[monthKey].forEach((extrinsic, index) => {
            // Main row
            html += '<tr>';
            html += `<td>${extrinsic.formattedTime || extrinsic.timestamp}</td>`;
            html += `<td>${extrinsic.method.method}</td>`;

            let amount = extrinsic.amount.toFixed(2);
            let totalAmount = extrinsic.totalAmount.toFixed(2);
            let detailsContent = {
                blockId: extrinsic.blockId, // Add blockId to details
                pallet: extrinsic.pallet,
                method: extrinsic.method.method,
                subpallet: extrinsic.method.pallet,
            };

            html += `<td class="has-text-right">${amount}</td>`;
            html += `<td class="has-text-right">${totalAmount}</td>`;

            const detailsId = `extrinsic-details-${monthKey}-${index}`;
            html += `<td><button class="button is-small toggle-details" data-target="${detailsId}">&gt;</button></td>`;
            html += '</tr>';

            // Details row (hidden by default)
            html += `<tr id="${detailsId}" class="details-row" style="display: none;">`;
            html += `<td colspan="4"><pre class="extrinsic-details">${JSON.stringify(detailsContent, null, 2)}</pre></td>`;
            html += '</tr>';
        });
    });

    html += '</tbody></table>';
    return html;
}

function extractBalancesFromBlocks(blocks, address, balanceAt) {
    const balances = [];

    // Go through all blocks and collect extrinsics
    blocks.forEach((block) => {
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
            extrinsicArray.forEach((extrinsic) => {
                if (extrinsic?.method.pallet) {
                    if (
                        palletName === 'paraInclusion' ||
                        palletName === 'staking' ||
                        (palletName === 'utility' && extrinsic.method.method === 'Rewarded')
                    ) {
                        return; // Skip paraInclusion extrinsics
                    }

                    let amount = 0.0;
                    if (extrinsic.method.pallet === 'balances' && extrinsic.method.method === 'Transfer') {
                        amount = parseFloat(extrinsic.data[2]);
                        if (address === extrinsic.data[0]) {
                            amount = -amount;
                        }
                    } else if (extrinsic.method.pallet === 'balances' && extrinsic.method.method === 'Deposit') {
                        amount = parseFloat(extrinsic.data[1]);
                    } else if (extrinsic.method.pallet === 'balances' && extrinsic.method.method === 'Withdraw') {
                        amount = -parseFloat(extrinsic.data[1]);
                    }

                    amount = amount / 10 / 1000 / 1000 / 1000;

                    balances.push({
                        timestamp,
                        blockId,
                        pallet: palletName,
                        method: extrinsic.method,
                        amount: amount,
                        totalAmount: amount + balanceAt,
                    });
                }
            });
        });
    });

    return balances;
}

// Function to add toggle listeners for extrinsic details
function addExtrinsicToggleListeners() {
    document.querySelectorAll('.toggle-details').forEach((button) => {
        button.addEventListener('click', function () {
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
async function fetchBalances(balancesUrl) {
    const searchInput = document.getElementById('search-address');
    const address = searchInput.value.trim();
    if (!address) {
        return;
    }

    const url = new URL(window.location.href);
    const params = new URLSearchParams(url.search).toString();
    const frontendUrl = `/fe/balances?${params}`;

    const response = await fetch(frontendUrl);
    if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
    }
    const textRaw = await response.text();
    const result = await JSON.parse(textRaw);

    const resultDiv = document.getElementById('balances-result');
    const dataDiv = document.getElementById('balances-data');
    const graphDiv = document.getElementById('balances-graph');
    const summaryDiv = document.getElementById('balances-summary');

    resultDiv.classList.remove('is-hidden');

    if (result && Array.isArray(result) && result.length > 0) {
        const firstResult = result[0];
        const lastResult = result[result.length - 1];
        const firstBalance = await getAccountAt(address, firstResult.number);
        const lastBalance = await getAccountAt(address, lastResult.number);

        const summaryHtml = `
<h4 class="title is-4">Balance</h4>
<table class="table is-fullwidth is-striped">
  <thead>
    <tr>
      <th>${firstBalance.symbol}<th>
      <th class="has-text-right">From ${firstResult.timestamp}<th>
      <th class="has-text-right">To ${lastResult.timestamp}<th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <th>Free<th>
      <td class="has-text-right">${firstBalance.free}<td>
      <td class="has-text-right">${lastBalance.free}<td>
    </tr>
    <tr>
      <th>Reserved<th>
      <td class="has-text-right">${firstBalance.reserved}<td>
      <td class="has-text-right">${lastBalance.reserved}<td>
    </tr>
    <tr>
      <th>Frozen<th>
      <td class="has-text-right">${firstBalance.frozen}<td>
      <td class="has-text-right">${lastBalance.frozen}<td>
    </tr>
  </tbody>
</table>
`;

        summaryDiv.innerHTML = summaryHtml;

        const balances = extractBalancesFromBlocks(result, address, firstBalance.free);
        // Format timestamps for display
        balances.forEach((extrinsic) => {
            if (extrinsic.timestamp !== 'N/A') {
                extrinsic.formattedTime = formatTimestamp(extrinsic.timestamp);
            }
        });

        // Group extrinsics by month
        const balancesByMonth = groupBalancesByMonth(balances);

        // Create graph data
        const graphData = buildBalanceGraphData(balances);
        createBalanceGraph(graphData, graphDiv, address);

        // Render the table
        dataDiv.innerHTML = renderBalancesTable(balancesByMonth);

        // Add toggle listeners for extrinsic details
        addExtrinsicToggleListeners();
    } else {
        dataDiv.innerHTML = '<p>No balance data found for this address.</p>';
        graphDiv.innerHTML = '';
    }
}

document.addEventListener('DOMContentLoaded', () => {
    initAddresses('search-balances', '/balances.html', fetchBalances);
});
