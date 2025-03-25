// stats.js - Statistics-related functionality for DotIDX

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
    const data = await response.json();

    monthlyResult.classList.remove("is-hidden");

    // Format the monthly stats data
    let html = '<div class="content">';
    html += '<h4 class="subtitle">Monthly Processing Statistics</h4>';
    html += '<table class="table is-striped is-fullwidth">';
    html += `
            <thead>
                <tr>
                    <th>Month</th>
                    <th>Blocks Processed</th>
                    <th>First Block</th>
                    <th>Last Block</th>
                </tr>
            </thead>
            <tbody>
        `;

    // Sort months chronologically
    const sortedMonths = Object.keys(data.data).sort((a, b) => {
      const [yearA, monthA] = a.split("-").map(Number);
      const [yearB, monthB] = b.split("-").map(Number);
      if (yearA !== yearB) return yearA - yearB;
      return monthA - monthB;
    });

    sortedMonths.forEach((month) => {
      const stats = data.data[month];
      html += `
                <tr>
                    <td>${stats.date}</td>
                    <td>${stats.count}</td>
                    <td>${stats.min_block}</td>
                    <td>${stats.max_block}</td>
                </tr>
            `;
    });

    html += "</tbody>";
    html += "</table>";
    html += "</div>";

    monthlyData.innerHTML = html;
  } catch (error) {
    showError("monthly", error.message);
  }
}

// Export functions
export { fetchCompletionRate, fetchMonthlyStats };
