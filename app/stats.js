// import Plotly from "plotly.js-dist-min";
import { updateFooter, updateNav } from './components.js';
import { showError } from './misc.js';

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

    // Format the completion rate data
    let html = '<div class="content">';
    html += '<table class="table is-striped is-fullwidth">';
    html += `
            <thead>
                <tr>
                    <th>Relay Chain</th>
                    <th>Chain</th>
                    <th class="has-text-right">Completion Rate (%)</th>
                    <th class="has-text-right">HeadID</th>
                </tr>
            </thead>
            <tbody>
        `;
    datas.forEach((data) => {
        html += `
                <tr>
                    <td>${data.RelayChain}</td>
                    <td>${data.Chain}</td>
                    <td class="has-text-right">${data.percent_completion.toFixed(2)}</td>
                    <td class="has-text-right">${data.head_id.toLocaleString('en-US')}</td>
                </tr>
              `;
    });
    html += '</tbody>';
    html += '</table>';
    html += '</div>';

    completionData.innerHTML = html;
    return '';
}

// Function to fetch and display monthly statistics
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
    let html = '<div class="content">';
    html += '<div id="monthly-chart" style="width:100%; height:400px;"></div>';
    html += '</div>';

    monthlyData.innerHTML = html;
    monthlyResult.classList.remove('is-hidden');

    const plotDiv = document.getElementById('monthly-chart');

    const chains = new Set(datas.map((d) => d.Chain));
    const traces = [...chains].map((chain) => ({
        name: '#' + chain,
        x: datas.filter((d) => d.Chain == chain).map((d) => d.date),
        y: datas.filter((d) => d.Chain == chain).map((d) => d.count),
        type: 'bar',
    }));

    const layout = {
        title: 'Indexed blocks per month per chain',
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
            t: 50,
            pad: 4,
        },
    };

    Plotly.newPlot(plotDiv, traces, layout);

    return '';
}

async function initStats() {
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
