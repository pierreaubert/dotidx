// Helper function to show error messages
export function showError(section, message) {
  if (message) {
    console.error(`Error in ${section} section:`, message);
  } else {
    console.error(`Error in ${section} section: (no message)`);
  }
  const errorDiv = document.getElementById(`${section}-error`);
  if (errorDiv) {
    errorDiv.innerHTML = `<div class="notification is-danger">${message}</div>`;
    errorDiv.classList.remove("is-hidden");
  }
}

// Escape HTML to prevent XSS
export function escapeHtml(unsafe) {
  return unsafe
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

// Function to highlight address matches
export function highlightAddressMatches(text, address) {
  if (!address || typeof text !== "string" || typeof address !== "string") {
    return text;
  }

  // Create a case-insensitive regular expression to find all occurrences
  const regex = new RegExp(escapeRegExp(address), "gi");

  // Replace matches with highlighted version
  return text.replace(
    regex,
    (match) => `<span class="address-highlight">${match}</span>`,
  );
}

// Escape special characters in regex
export function escapeRegExp(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// Helper function to format JSON as a table
export function formatJsonToTable(data) {
  if (!data || typeof data !== "object") {
    return "<p>No data available.</p>";
  }

  // Check if it's an array of objects
  if (Array.isArray(data)) {
    if (data.length === 0) {
      return "<p>No data available.</p>";
    }

    // Create table headers from the first object's keys
    const keys = Object.keys(data[0]);
    let html = '<table class="table is-fullwidth is-hoverable result-table">';
    html += "<thead><tr>";

    keys.forEach((key) => {
      html += `<th>${key}</th>`;
    });

    html += "</tr></thead><tbody>";

    // Add rows for each object
    data.forEach((item) => {
      html += "<tr>";

      keys.forEach((key) => {
        const value = item[key];
        if (typeof value === "object" && value !== null) {
          html += `<td>${JSON.stringify(value)}</td>`;
        } else {
          html += `<td>${value}</td>`;
        }
      });

      html += "</tr>";
    });

    html += "</tbody></table>";
    return html;
  } else {
    // It's a single object, create a key-value table
    let html = '<table class="table is-fullwidth is-hoverable result-table">';
    html += "<thead><tr><th>Key</th><th>Value</th></tr></thead>";
    html += "<tbody>";

    Object.entries(data).forEach(([key, value]) => {
      html += "<tr>";
      html += `<td>${key}</td>`;

      if (typeof value === "object" && value !== null) {
        html += `<td>${JSON.stringify(value)}</td>`;
      } else {
        html += `<td>${value}</td>`;
      }

      html += "</tr>";
    });

    html += "</tbody></table>";
    return html;
  }
}
// Function to format timestamp as 'DD HH:MM'
export function formatTimestamp(timestamp) {
  if (timestamp === "N/A") return timestamp;

  try {
    const date = new Date(timestamp);
    const day = String(date.getDate()).padStart(2, "0");
    const hours = String(date.getHours()).padStart(2, "0");
    const minutes = String(date.getMinutes()).padStart(2, "0");
    return `${day} ${hours}:${minutes}`;
  } catch (e) {
      console.error("Error in formatTimestamp:", e);
    return timestamp;
  }
}
