import { updateIcons, updateFooter, updateNav } from './components.js';

function printCompletionRateRelayChain(name, datas) {
    let html = `<h4>${name}</h4>`;
    html += '<table class="table is-striped is-fullwidth">';
    html += `
            <thead>
                <tr>
                    <th>Chain</th>
                    <th class="has-text-right">%</th>
                    <th class="has-text-right">HeadID</th>
                </tr>
            </thead>
            <tbody>
        `;
    datas.forEach((data) => {
        if (data.RelayChain === name) {
            html += `
                <tr>
                    <td>${data.Chain}</td>
                    <td class="has-text-right">${data.percent_completion.toFixed(2)}</td>
                    <td class="has-text-right">${data.head_id.toLocaleString('en-US')}</td>
                </tr>
              `;
        }
    });
    html += '</tbody>';
    html += '</table>';
    return html;
}

// Function to fetch and display completion rate
async function fetchCompletionRate() {
    const completionData = document.getElementById('completion-data');
    const completionResult = document.getElementById('completion-result');

    if (!completionData || !completionResult) {
        return "Element dont't exists"; // Exit if elements don't exist
    }

    const response = await fetch('/fe/stats/completion_rate');
    if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
    }
    const datas = await response.json();

    completionResult.classList.remove('is-hidden');

    let html = '<div class="content">';
    html += printCompletionRateRelayChain('polkadot', datas);
    html += printCompletionRateRelayChain('kusama', datas);
    html += '</div>';
    completionData.innerHTML = html;
    return '';
}

// Function to fetch and display monthly statistics
function plotMonthlyStats(allDatas) {
    const plotDiv = document.getElementById('monthly-chart');

    function addTrace(relay) {
        const datas = allDatas.filter((d) => d.Relaychain === relay);
        const chains = new Set(datas.map((d) => d.Chain));
        return [...chains].map((chain) => ({
            name: '#' + chain + '.' + relay.slice(0, 3),
            x: datas.filter((d) => d.Chain == chain).map((d) => d.date),
            y: datas.filter((d) => d.Chain == chain).map((d) => d.count),
            type: 'bar',
        }));
    }

    let tracesPolkadot = addTrace('polkadot');
    let tracesKusama = addTrace('kusama');

    const traces = tracesPolkadot.concat(tracesKusama);

    const total = allDatas.map((d) => d.count).reduce((a, b) => a + b, 0);

    const layout = {
        title: {
            text: total + ' blocks',
        },
        xaxis: {
            title: 'Month',
            tickangle: -45,
        },
        yaxis: {
            title: 'Block Count per month',
            rangemode: 'tozero',
        },
        barmode: 'stack',
        legend: {
            orientation: 'h',
            y: -0.2,
        },
        margin: {
            l: 50,
            r: 50,
            b: 100,
            t: 80,
            pad: 4,
        },
    };

    Plotly.newPlot(plotDiv, traces, layout);
}

async function fetchMonthlyStats() {
    const monthlyData = document.getElementById('monthly-data');
    const monthlyResult = document.getElementById('monthly-result');

    if (!monthlyData || !monthlyResult) {
        return "Element dont't exists";
    }

    const response = await fetch('/fe/stats/per_month');
    if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
    }
    const datas = await response.json();

    // Create content and title
    monthlyData.innerHTML = '<div id="monthly-chart" style="width:100%; height:400px;"></div>';
    monthlyResult.classList.remove('is-hidden');

    plotMonthlyStats(datas);

    return '';
}

async function initStats() {
    await updateIcons();
    await updateNav();
    await updateFooter();

    const errCR = await fetchCompletionRate();
    const errMS = await fetchMonthlyStats();

    if (errCR !== '') {
        console.error('CompletionRate ' + errCR);
    }

    if (errMS !== '') {
        console.error('MonthlyStats ' + errMS);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    initStats();
});
