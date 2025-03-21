// stats.js - Statistics-related functionality for DotIDX

// Function to fetch and display completion rate
async function fetchCompletionRate() {
    const completionData = document.getElementById('completion-data');
    const completionResult = document.getElementById('completion-result');

    if (!completionData || !completionResult) {
        return; // Exit if elements don't exist
    }

    try {
        const response = await fetch('/stats/completion_rate');
        if (!response.ok) {
            throw new Error(`HTTP error ${response.status}`);
        }
        const data = await response.json();

        completionResult.classList.remove('is-hidden');

        // Format the completion rate data
        let html = '<div class="content">';
        html += `<p>Current completion rate: <strong>${data.rate.toFixed(2)}%</strong></p>`;
        html += '<table class="table is-striped is-fullwidth">';
        html += `
            <thead>
                <tr>
                    <th>Metric</th>
                    <th>Value</th>
                </tr>
            </thead>
            <tbody>
                <tr>
                    <td>Total Blocks Processed</td>
                    <td>${data.processed.toLocaleString()}</td>
                </tr>
                <tr>
                    <td>Target Blocks</td>
                    <td>${data.target.toLocaleString()}</td>
                </tr>
                <tr>
                    <td>Remaining Blocks</td>
                    <td>${data.remaining.toLocaleString()}</td>
                </tr>
            </tbody>
        `;
        html += '</table>';
        html += '</div>';

        completionData.innerHTML = html;
    } catch (error) {
        showError('completion', error.message);
    }
}

// Function to fetch and display monthly statistics
async function fetchMonthlyStats() {
    const monthlyData = document.getElementById('monthly-data');
    const monthlyResult = document.getElementById('monthly-result');

    if (!monthlyData || !monthlyResult) {
        return; // Exit if elements don't exist
    }

    try {
        const response = await fetch('/stats/per_month');
        if (!response.ok) {
            throw new Error(`HTTP error ${response.status}`);
        }
        const data = await response.json();

        monthlyResult.classList.remove('is-hidden');

        // Format the monthly stats data
        let html = '<div class="content">';
        html += '<h4 class="subtitle">Monthly Processing Statistics</h4>';
        html += '<table class="table is-striped is-fullwidth">';
        html += `
            <thead>
                <tr>
                    <th>Month</th>
                    <th>Blocks Processed</th>
                    <th>Extrinsics Processed</th>
                </tr>
            </thead>
            <tbody>
        `;

        // Sort months chronologically
        const sortedMonths = Object.keys(data).sort((a, b) => {
            const [yearA, monthA] = a.split('-').map(Number);
            const [yearB, monthB] = b.split('-').map(Number);
            if (yearA !== yearB) return yearA - yearB;
            return monthA - monthB;
        });

        sortedMonths.forEach(month => {
            const stats = data[month];
            html += `
                <tr>
                    <td>${month}</td>
                    <td>${stats.blocks.toLocaleString()}</td>
                    <td>${stats.extrinsics.toLocaleString()}</td>
                </tr>
            `;
        });

        html += '</tbody>';
        html += '</table>';
        html += '</div>';

        monthlyData.innerHTML = html;
    } catch (error) {
        showError('monthly', error.message);
    }
}

// Helper function to show error messages
function showError(section, message) {
    console.error(`Error in ${section} section:`, message);
    const errorDiv = document.getElementById(`${section}-error`);
    if (errorDiv) {
        errorDiv.innerHTML = `<div class="notification is-danger">${message}</div>`;
        errorDiv.classList.remove('is-hidden');
    }
}

// Export functions
export {
    fetchCompletionRate,
    fetchMonthlyStats,
    showError
};
