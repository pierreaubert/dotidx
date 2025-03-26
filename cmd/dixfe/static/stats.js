// import Plotly from "plotly.js-dist-min";
import { showError } from "./misc.js";

// Function to fetch and display completion rate
async function fetchCompletionRate() {
  const completionData = document.getElementById("completion-data");
  const completionResult = document.getElementById("completion-result");

  if (!completionData || !completionResult) {
    return; // Exit if elements don't exist
  }

  try {
    const response = await fetch("/stats/completion_rate");
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status}`);
    }
    const datas = await response.json();

    completionResult.classList.remove("is-hidden");

    // Format the completion rate data
    let html = '<div class="content">';
    html += '<table class="table is-striped is-fullwidth">';
    html += `
            <thead>
                <tr>
                    <th>Relay Chain</th>
                    <th>Chain</th>
                    <th>Completion Rate (%)</th>
                    <th>HeadID</th>
                </tr>
            </thead>
            <tbody>
        `;
      datas.forEach( (data) => {
	  html += `
                <tr>
                    <td>${data.RelayChain}</td>
                    <td>${data.Chain}</td>
                    <td>${data.percent_completion.toFixed(2)}</td>
                    <td>${data.head_id}</td>
                    <td>
                </tr>
              `;
      });
      html += "</tbody>";
      html += "</table>";
      html += "</div>";

      completionData.innerHTML = html;
  } catch (error) {
    showError("completion", error.message);
  }
}

// Function to fetch and display monthly statistics
async function fetchMonthlyStats() {
  const monthlyData = document.getElementById("monthly-data");
  const monthlyResult = document.getElementById("monthly-result");

  if (!monthlyData || !monthlyResult) {
    return; // Exit if elements don't exist
  }

  try {
    const response = await fetch("/stats/per_month");
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status}`);
    }
    const datas = await response.json();

    // Create content and title
    let html = '<div class="content">';
    html += '<div id="monthly-chart" style="width:100%; height:400px;"></div>';
    html += '</div>';

    monthlyData.innerHTML = html;
    monthlyResult.classList.remove("is-hidden");

    const plotDiv = document.getElementById('monthly-chart');

    const chains = new Set(datas.map((d) => d.Chain));
    const traces = [...chains].map((chain) => ({
        name: '#' + chain,
        x: datas.filter((d) => d.Chain == chain).map((d)=> d.date),
        y: datas.filter((d) => d.Chain == chain).map((d)=> d.count),
        type: 'bar',
      }));

    const layout = {
      title: 'Indexed blocks per month per chain',
      xaxis: {
        title: 'Month',
        tickangle: -45
      },
      yaxis: {
        title: 'Block Count per month',
        rangemode: 'tozero'
      },
     barmode: 'stack',
      legend: {
        orientation: 'h',
        y: -0.2
      },
      margin: {
        l: 50,
        r: 50,
        b: 100,
        t: 50,
        pad: 4
      }
    };

    Plotly.newPlot(plotDiv, traces, layout);

  } catch (error) {
    showError("monthly", error.message);
  }
}

// Export functions
export { fetchCompletionRate, fetchMonthlyStats };
