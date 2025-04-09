// import {Plotly} from "plotly.js-dist-min";
import { showError, formatTimestamp } from './misc.js';
import { initAddresses, getAddress } from './assets.js';

function buildStakingGraphData(stakingData) {
    const transactionsByDay = {};

    // Process extrinsics to collect time series data
    stakingData.forEach((extrinsic) => {
        if (extrinsic.timestamp === 'N/A') {
            return; // Skip entries without valid timestamps
        }

        const date = new Date(extrinsic.timestamp);
        const dayKey = date.toISOString().split('T')[0];
        let amount = extrinsic.totalAmount;

        if (!transactionsByDay[dayKey]) {
            transactionsByDay[dayKey] = {
                date: new Date(dayKey), // Start of the day
                rewards: 0,
                deposits: 0,
                bonded: 0,
                count: 0,
            };
        }

        if (extrinsic.method.method == 'Rewarded') {
            transactionsByDay[dayKey].rewards += amount;
        } else if (extrinsic.method.method == 'Bonded' || extrinsic.method.method == 'Unbonded') {
            transactionsByDay[dayKey].bonded += amount;
        } else if (extrinsic.method.method == 'Withdrawn' || extrinsic.method.method == 'Deposit') {
            transactionsByDay[dayKey].deposits += amount;
        }
        transactionsByDay[dayKey].count += 1;
    });

    // Convert the grouped data to an array
    const graphData = Object.values(transactionsByDay);

    // Sort by date (oldest first for cumulative graph)
    return graphData.sort((a, b) => a.date - b.date);
}

// Function to create plotly graph
function createStakingGraph(graphData, graphDiv, address) {
    if (graphData.length === 0) {
        graphDiv.innerHTML = '<p class="has-text-centered">No transaction data available for plotting.</p>';
        return;
    }

    let cummulativeRewards = 0;
    const rewards = graphData.map((item) => {
        cummulativeRewards += item.rewards;
        return {
            x: item.date,
            y: item.rewards,
            z: cummulativeRewards,
            text: `Date: ${item.date.toLocaleDateString()}<br>Staking: ${item.rewards}<br>Day change: ${item.rewards}<br>Transactions: ${item.count}`,
        };
    });

    let pos = -1;
    const avgRewards = rewards.map((item) => {
        const window = 5;
        pos = pos + 1;
        const slice = rewards.slice(Math.max(0, pos - window), Math.min(pos + window, rewards.length));
        const total = slice.map((it) => it.y).reduce((acc, val) => acc + val);
        const avg = (total * 1000 * 3600 * 24) / (slice[slice.length - 1].x - slice[0].x);
        return {
            x: item.x,
            y: avg,
        };
    });

    const deposits = graphData.map((item) => ({
        x: item.date,
        y: item.deposits,
        text: `Date: ${item.date.toLocaleDateString()}<br>Deposits: ${item.deposits.toFixed(4)}`,
    }));

    const bonded = graphData.map((item) => ({
        x: item.date,
        y: item.bonded,
        text: `Date: ${item.date.toLocaleDateString()}<br>Bonded: ${item.bonded.toFixed(4)}`,
    }));

    // Create the plotly data array
    const plotData = [
        {
            type: 'bar',
            name: 'Daily',
            x: rewards.map((p) => p.x),
            y: rewards.map((p) => p.y),
            hoverinfo: rewards.map((p) => '%{x}<br>%{y}'),
        },
        {
            type: 'lines',
            name: 'Averaged (10d)',
            line: { width: 8 },
            x: avgRewards.map((it) => it.x),
            y: avgRewards.map((it) => it.y),
        },
        {
            type: 'scatter',
            name: 'Cummulative',
            mode: 'lines',
            line: { width: 8 },
            markers: { size: 10 },
            x: rewards.map((p) => p.x),
            y: rewards.map((p) => p.z),
            marker: { color: 'rgba(0, 200, 0, 0.7)' },
            hoverinfo: rewards.map((p) => '%{x}<br>%{y}'),
            yaxis: 'y2',
        },
    ];

    const text = `Cummulative rewards over time: ${cummulativeRewards.toFixed(0)} DOTs`;
    document.getElementById('staking-summary').textContent = text;

    // Configure the layout
    const layout = {
        legend: {
            orientation: 'h',
            y: -0.2,
        },
        hovermode: 'closest',
        xaxis: {
            title: { text: 'Date' },
        },
        yaxis: {
            title: { text: 'Daily' },
            tickformat: '.0f',
        },
        yaxis2: {
            title: { text: 'Cummulative' },
            side: 'right',
            tickformat: '.0f',
            overlaying: 'y',
        },
        margin: {
            l: 60,
            r: 60,
            b: 60,
            t: 80,
            pad: 2,
        },
    };

    // Create the graph
    Plotly.newPlot(graphDiv, plotData, layout, { responsive: true });
}

// Function to group extrinsics by month
function groupStakingsByMonth(allExtrinsics) {
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

function renderStakingsRewardsTable(extrinsicsByMonth, flip) {
    const sortedMonths = Object.keys(extrinsicsByMonth).sort((a, b) => {
        // Handle 'Unknown' specially
        if (a === 'Unknown') return 1;
        if (b === 'Unknown') return -1;
        return b.localeCompare(a); // Descending order
    });

    let html = '<table class="table is-fullwidth is-striped is-hoverable result-table">';
    html += `<thead>
          <tr>
            <th>Timestamp</th>
            <th>Chain</th>
            <th>Method</th>
            <th class="has-text-right">Amount (DOT)</th>
            <th>Details</th>
            </tr>
        </thead>`;
    html += '<tbody>';

    sortedMonths.forEach((monthKey) => {
        const monthName =
            monthKey === 'Unknown'
                ? 'Unknown Date'
                : new Date(`${monthKey}-01`).toLocaleString('default', {
                      year: 'numeric',
                      month: 'long',
                  });

        let first = true;

        extrinsicsByMonth[monthKey].forEach((extrinsic, index) => {
            let doit = extrinsic.method.method === 'Rewarded';
            if (flip) {
                doit = !doit;
            }

            if (doit) {
                if (first) {
                    html += `<tr class="month-header"><td colspan="4"><strong>${monthName}</strong></td></tr>`;
                    first = false;
                }

                html += '<tr>';
                html += `  <td>${extrinsic.formattedTime || extrinsic.timestamp}</td>`;
                html += `  <td>${extrinsic.chain}</td>`;
                html += `  <td>${extrinsic.method.method}</td>`;

                let amount = extrinsic.totalAmount.toFixed(2);
                let detailsContent = {
                    blockId: extrinsic.blockId, // Add blockId to details
                    pallet: extrinsic.pallet,
                    method: extrinsic.method.method,
                    subpallet: extrinsic.method.pallet,
                };

                html += `  <td class="has-text-right">${amount}</td>`;

                const detailsId = `extrinsic-details-${monthKey}-${index}`;
                html += `  <td><button class="button is-small toggle-details" data-target="${detailsId}">&gt;</button></td>`;
                html += '</tr>';

                html += `<tr id="${detailsId}" class="details-row" style="display: none;">`;
                html += `<td colspan="4"><pre class="extrinsic-details">${JSON.stringify(detailsContent, null, 2)}</pre></td>`;
                html += '</tr>';
            }
        });
    });

    html += '</tbody></table>';
    return html;
}

function extractStakingsFromBlocks(results, address) {
    const stakings = [];
    for (const [relay, chains] of Object.entries(results)) {
        for (const [chain, blocks] of Object.entries(chains)) {
            if (blocks == undefined) {
                continue;
            }
            blocks.forEach((block) => {
                if (!block.extrinsics) {
                    return;
                }

                const timestamp = block.timestamp || 'N/A';
                const blockId = block.number || 'N/A';

                if (block.extrinsics['staking'] != undefined) {
                    block.extrinsics['staking'].forEach((extrinsic) => {
                        if (extrinsic?.method.pallet == 'staking') {
                            let amount = 0.0;
                            if (extrinsic.method.method === 'Transfer') {
                                amount = parseFloat(extrinsic.data[2]);
                                if (address === extrinsic.data[0]) {
                                    amount = -amount;
                                }
                            } else if (extrinsic.method.method === 'Deposit') {
                                amount = parseFloat(extrinsic.data[1]);
                            } else if (extrinsic.method.method === 'Withdrawn') {
                                amount = -parseFloat(extrinsic.data[1]);
                            } else if (extrinsic.method.method === 'Rewarded') {
                                amount = parseFloat(extrinsic.data[2]);
                            } else if (extrinsic.method.method === 'Unbonded') {
                                amount = -parseFloat(extrinsic.data[1]);
                            } else if (extrinsic.method.method === 'Bonded') {
                                amount = parseFloat(extrinsic.data[1]);
                            } else {
                                console.log('TODO: ' + extrinsic.method.pallet + ' ' + extrinsic.method.method);
                            }

                            amount = amount / 10 / 1000 / 1000 / 1000;

                            stakings.push({
                                timestamp: timestamp,
                                relay: relay,
                                chain: chain,
                                blockId: blockId,
                                pallet: 'staking',
                                method: extrinsic.method,
                                totalAmount: amount,
                            });
                        }
                    });
                }

                if (block.extrinsics['utility'] != undefined) {
                    block.extrinsics['utility'].forEach((extrinsic) => {
                        if (extrinsic?.method.pallet == 'staking') {
                            let amount = 0.0;
                            if (extrinsic.method.method === 'Transfer') {
                                amount = parseFloat(extrinsic.data[2]);
                                if (address === extrinsic.data[0]) {
                                    amount = -amount;
                                }
                            } else if (extrinsic.method.method === 'Deposit') {
                                amount = parseFloat(extrinsic.data[1]);
                            } else if (extrinsic.method.method === 'Withdrawn') {
                                amount = -parseFloat(extrinsic.data[1]);
                            } else if (extrinsic.method.method === 'Rewarded') {
                                amount = parseFloat(extrinsic.data[2]);
                            } else if (extrinsic.method.method === 'Unbonded') {
                                amount = -parseFloat(extrinsic.data[1]);
                            } else if (extrinsic.method.method === 'Bonded') {
                                amount = parseFloat(extrinsic.data[1]);
                            } else {
                                console.log('TODO: ' + extrinsic.method.pallet + ' ' + extrinsic.method.method);
                            }

                            amount = amount / 10 / 1000 / 1000 / 1000;

                            stakings.push({
                                timestamp: timestamp,
                                blockId: blockId,
                                relay: relay,
                                chain: chain,
                                pallet: 'staking',
                                method: extrinsic.method,
                                totalAmount: amount,
                            });
                        }
                    });
                }
            });
        }
    }
    return stakings;
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

// Function to fetch stakings
async function fetchStaking() {
    const address = getAddress();
    if (!address) {
        return;
    }

    const url = new URL(window.location.href);
    const params = new URLSearchParams(url.search).toString();
    const frontendUrl = `/fe/staking?${params}`;

    const response = await fetch(frontendUrl);
    if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
    }
    const textRaw = await response.text();
    const result = JSON.parse(textRaw);

    const resultDiv = document.getElementById('staking-results');
    const othersDiv = document.getElementById('staking-others');
    const rewardsDiv = document.getElementById('staking-rewards');
    const graphDiv = document.getElementById('staking-graph');

    resultDiv.classList.remove('is-hidden');

    if (result) {
        const stakings = extractStakingsFromBlocks(result, address);

        stakings.forEach((extrinsic) => {
            if (extrinsic.timestamp !== 'N/A') {
                extrinsic.formattedTime = formatTimestamp(extrinsic.timestamp);
            }
        });

        const stakingsByMonth = groupStakingsByMonth(stakings);

        const graphData = buildStakingGraphData(stakings);
        createStakingGraph(graphData, graphDiv, address);

        othersDiv.innerHTML = renderStakingsRewardsTable(stakingsByMonth, true);
        rewardsDiv.innerHTML = renderStakingsRewardsTable(stakingsByMonth, false);

        addExtrinsicToggleListeners();
    } else {
        resultDiv.innerHTML = '<p>No staking data found for this address.</p>';
        graphDiv.innerHTML = '';
        othersDiv.innerHTML = '';
        rewardsDiv.innerHTML = '';
    }
}

document.addEventListener('DOMContentLoaded', () => {
    initAddresses('search-staking', '/staking.html', fetchStaking);
});
