// import Plotly from "plotly.js-dist-min";
import { formatTimestamp } from './misc.js';
import { default_balance, getAccountAt } from './accounts.js';
import { initAddresses, getAddress } from './assets.js';

// Function to build balance graph data
function buildBalanceGraphData(balances) {
    const transactionsByDay = {};

    // Process extrinsics to collect time series data
    balances.forEach((balance) => {
        const date = new Date(balance.timestamp);
        const dayKey = date.toISOString().split('T')[0];
        let amount = balance.amount;
        let totalAmount = balance.totalAmount;

        if (!transactionsByDay[dayKey]) {
            transactionsByDay[dayKey] = {
                date: new Date(dayKey), // Start of the day
                relay: balance.relay,
                chain: balance.chain,
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
            c: item.chain,
        };
    });

    // Create data for deposits and withdrawals
    const deposits = graphData.map((item) => ({
        x: item.date,
        y: item.deposits,
        c: item.chain,
        text: `Date: ${item.date.toLocaleDateString()}<br>Deposits: +${item.deposits}`,
    }));

    const withdrawals = graphData.map((item) => ({
        x: item.date,
        y: item.withdrawals,
        c: item.chain,
        text: `Date: ${item.date.toLocaleDateString()}<br>Withdrawals: -${item.withdrawals}`,
    }));

    // Create the plotly data array
    const plotData = [
        {
            type: 'scatter',
            mode: 'lines+markers',
            name: 'Balance ' + address,
            x: balanceSeries.map((p) => p.x),
            y: balanceSeries.map((p) => p.y),
            color: balanceSeries.map((p) => p.c),
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
            // log axis so invert withdrawls
            y: withdrawals.map((p) => -p.y),
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
        yaxis: {
            title: { text: 'Balance' },
            tickformat: '.0f',
            type: 'linear',
            showline: true,
        },
        yaxis2: {
            title: { text: 'Daily Activity (log scale)' },
            titlefont: { color: 'rgb(148, 103, 189)' },
            tickfont: { color: 'rgb(148, 103, 189)' },
            overlaying: 'y',
            side: 'right',
            tickformat: '.0f',
            type: 'log',
            showline: true,
        },
        margin: {
            l: 80,
            r: 80,
            b: 60,
            t: 40,
            pad: 2,
        },
    };

    // Create the graph
    Plotly.newPlot(graphDiv, plotData, layout, { responsive: true });
}

// Function to group extrinsics by month
function groupBalancesByMonth(balances) {
    const balancesByMonth = {};

    balances.forEach((balance) => {
        if (balance.timestamp !== 'N/A') {
            try {
                // Parse the timestamp
                const date = new Date(balance.timestamp);

                // Create a month key in YYYY-MM format
                const monthKey = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;

                // Initialize the month array if it doesn't exist
                if (!balancesByMonth[monthKey]) {
                    balancesByMonth[monthKey] = [];
                }

                // Add the balance to the month
                balancesByMonth[monthKey].push(balance);
            } catch (e) {
                console.error('invalid timestamp', e);
                if (!balancesByMonth['Unknown']) {
                    balancesByMonth['Unknown'] = [];
                }
                balancesByMonth['Unknown'].push(balance);
            }
        } else {
            if (!balancesByMonth['Unknown']) {
                balancesByMonth['Unknown'] = [];
            }
            balancesByMonth['Unknown'].push(balance);
        }
    });

    return balancesByMonth;
}

function renderBalancesTable(balancesByMonth) {
    let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
    html += `
	 <thead>
           <tr>
             <th>Chain</th>
             <th>Timestamp</th>
             <th>Method</th>
             <th class="has-text-right">Amount (DOT)</th>
             <th class="has-text-right">Balance (DOT)</th>
             <th>Details</th></tr>
         </thead>
`;
    html += '<tbody>';

    // Sort month keys in descending order (newest first)
    const sortedMonths = Object.keys(balancesByMonth).sort((a, b) => {
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

        // Add balances for this month
        const sortedDays = balancesByMonth[monthKey].sort((a, b) => {
            return b.timestamp.localeCompare(a.timestamp);
        });

        sortedDays.forEach((balance, index) => {
            // Main row
            html += '<tr>';
            html += `<td>${balance.relay}/${balance.chain}</td>`;
            html += `<td>${balance.formattedTime || balance.timestamp}</td>`;
            html += `<td>${balance.method}</td>`;

            let amount = balance.amount.toFixed(2);
            let totalAmount = balance.totalAmount.toFixed(2);
            let detailsContent = {
                blockId: balance.blockId, // Add blockId to details
                pallet: balance.pallet,
                method: balance.method,
                subpallet: balance.subpallet,
            };

            html += `<td class="has-text-right">${amount}</td>`;
            html += `<td class="has-text-right">${totalAmount}</td>`;

            const detailsId = `balance-details-${monthKey}-${index}`;
            html += `<td><button class="button is-small toggle-details" data-target="${detailsId}">&gt;</button></td>`;
            html += '</tr>';

            // Details row (hidden by default)
            html += `<tr id="${detailsId}" class="details-row" style="display: none;">`;
            html += `<td colspan="4"><pre class="balance-details">${JSON.stringify(detailsContent, null, 2)}</pre></td>`;
            html += '</tr>';
        });
    });

    html += '</tbody></table>';
    return html;
}

function extractBalancesFromBlocks(results, address, balanceAt) {
    const balances = [];

    // Go through all blocks and collect extrinsics
    for (const [relay, chains] of Object.entries(results)) {
        for (const [chain, blocks] of Object.entries(chains)) {
            if (blocks == undefined) {
                continue;
            }
            blocks.forEach((block) => {
                if (!block.extrinsics || typeof block.extrinsics !== 'object') {
                    return;
                }

                const timestamp = block.timestamp || 'N/A';
                const blockId = block.number || 'N/A';

                block.extrinsics.forEach((extrinsic) => {
                    extrinsic.events.forEach((event) => {
                        if (event?.method.pallet === 'balances') {
                            let amount = 0.0;
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
                                default:
                                    console.log('TODO: ' + event.method.pallet + ' ' + event.method.method);
                            }
                            amount = amount / 10 / 1000 / 1000 / 1000;
                            balances.push({
                                relay: relay,
                                chain: chain,
                                address: address,
                                timestamp: timestamp,
                                blockId: blockId,
                                pallet: event.method.pallet,
                                method: event.method.method,
                                amount: amount,
                                totalAmount: amount + balanceAt.get(relay).get(chain),
                            });
                        }
                    });
                });
            });
        }
    }
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

async function balancesSummary(address, results) {
    let summaryHtml = `
<h4 class="title is-4">Balance</h4>
<table class="table is-fullwidth is-striped">
  <thead>
    <tr>
      <th>Account</th>
      <th>Token</th>
      <th>State</th>
      <th class="has-text-right">Now</th>
    </tr>
  </thead>
  <tbody>
`;
    let balanceAt = new Map();
    for (const [relay, chains] of Object.entries(results)) {
        let totalFree = 0.0;
        balanceAt.set(relay, new Map());
        for (const [chain, result] of Object.entries(chains)) {
            let firstBalance = default_balance;
            let lastBalance = default_balance;
            if (result && Array.isArray(result) && result.length > 0) {
                const firstResult = result[0];
                const lastResult = result[result.length - 1];
                firstBalance = await getAccountAt(relay, chain, address, firstResult.number);
                lastBalance = await getAccountAt(relay, chain, address, lastResult.number);
            }
            const nowBalance = await getAccountAt(relay, chain, address, '');
            totalFree += nowBalance.free;
            balanceAt.get(relay).set(chain, nowBalance.free);
            // FREE
            if (firstBalance.free + lastBalance.free + nowBalance.free > 0) {
                summaryHtml += `
<tr>
  <th>${relay}/${chain}</th>
  <th>${nowBalance.symbol}</th>
  <th>Free</th>
  <td class="has-text-right">${nowBalance.free}</td>
</tr>`;
            }
            // RESERVED
            if (firstBalance.reserved + lastBalance.reserved + nowBalance.reserved > 0) {
                summaryHtml += `
<tr>
  <th></th>
  <th>${nowBalance.symbol}</th>
  <th>Reserved</th>
  <td class="has-text-right">${nowBalance.reserved}</td>
</tr>`;
            }
            if (firstBalance.frozen + lastBalance.frozen + nowBalance.frozen > 0) {
                summaryHtml += `
<tr>
  <th></th>
  <th>${nowBalance.symbol}</th>
  <th>Frozen</th>
  <td class="has-text-right">${nowBalance.frozen}</td>
</tr>
    `;
            }
        }
        if (totalFree > 0) {
            summaryHtml += `
<tr>
    <th>${relay}: total free</th>
    <th></th>
    <th></th>
    <th class="has-text-right">${totalFree}</th>
</tr>`;
        }
    }
    summaryHtml += `
  </tbody>
</table>
`;
    return [summaryHtml, balanceAt];
}

// Function to fetch balances
async function fetchBalances(_balanceUrl) {
    const address = getAddress();
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

    const resultDiv = document.getElementById('balances-results');
    const dataDiv = document.getElementById('balances-data');
    const graphDiv = document.getElementById('balances-graph');
    const summaryDiv = document.getElementById('balances-summary');

    resultDiv.classList.remove('is-hidden');

    const [summaryHtml, balanceAt] = await balancesSummary(address, result);
    summaryDiv.innerHTML = summaryHtml;
    const balances = extractBalancesFromBlocks(result, address, balanceAt);
    // Format timestamps for display
    balances.forEach((extrinsic) => {
        if (extrinsic.timestamp !== 'N/A') {
            extrinsic.formattedTime = formatTimestamp(extrinsic.timestamp);
        }
    });

    // Create graph data
    const graphData = buildBalanceGraphData(balances);
    createBalanceGraph(graphData, graphDiv, address);

    // Group extrinsics by month
    const balancesByMonth = groupBalancesByMonth(balances);

    // Render the table
    dataDiv.innerHTML = renderBalancesTable(balancesByMonth);

    // Add toggle listeners for extrinsic details
    addExtrinsicToggleListeners();
}

document.addEventListener('DOMContentLoaded', () => {
    initAddresses('search-balances', '/balances.html', fetchBalances);
});
