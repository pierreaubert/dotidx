import {showError, escapeHtml, highlightAddressMatches } from "./misc.js";

function renderBlockContent(content, searchAddress) {
  if (content === null || content === undefined) {
    return '<span class="has-text-grey">null</span>';
  }

  if (Array.isArray(content)) {
    if (content.length === 0) {
      return '<span class="has-text-grey">(empty array)</span>';
    }

    let html =
      '<table class="table is-fullwidth is-bordered is-narrow nested-table">';
    html += "<thead><tr><th>Index</th><th>Value</th></tr></thead>";
    html += "<tbody>";

    content.forEach((item, index) => {
      html += `<tr><td>${index}</td><td>${renderBlockContent(item, searchAddress)}</td></tr>`;
    });

    html += "</tbody></table>";
    return html;
  }

  if (typeof content === "object") {
    if (Object.keys(content).length === 0) {
      return '<span class="has-text-grey">(empty object)</span>';
    }

    let html =
      '<table class="table is-fullwidth is-bordered is-narrow nested-table">';
    html += "<thead><tr><th>Property</th><th>Value</th></tr></thead>";
    html += "<tbody>";

    Object.entries(content).forEach(([key, value]) => {
      html += `<tr><td>${key}</td><td>${renderBlockContent(value, searchAddress)}</td></tr>`;
    });

    html += "</tbody></table>";
    return html;
  }

  if (typeof content === "string") {
    const escapedContent = escapeHtml(content);
    const highlightedContent = highlightAddressMatches(
      escapedContent,
      searchAddress,
    );

    if (content.length > 50) {
      return `<span class="is-family-monospace break-word">${highlightedContent}</span>`;
    }
    return `<span class="is-family-monospace">${highlightedContent}</span>`;
  }

  return String(content);
}

async function fetchBlocks() {
  const searchInput = document.getElementById("search-address");
  const address = searchInput.value.trim();
  if (!address) {
    alert("Please enter an address");
    return;
  }

  // Store data in a global variable for pagination
  window.blockData = window.blockData || {
    blocks: [],
    currentPage: 0,
    pageSize: 5,
  };

  try {
    const response = await fetch(
      `/address2blocks?address=${encodeURIComponent(address)}`,
    );
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status}`);
    }
    const data = await response.json();

    const resultDiv = document.getElementById("blocks-result");
    const dataDiv = document.getElementById("blocks-data");

    resultDiv.classList.remove("is-hidden");

    if (data && Array.isArray(data) && data.length > 0) {
      // Filter out paraInclusion extrinsics
      const filteredData = data.map((block) => {
        // Create a shallow copy of the block
        const filteredBlock = { ...block };

        // Filter out paraInclusion extrinsics if they exist
        if (
          filteredBlock.extrinsics &&
          typeof filteredBlock.extrinsics === "object"
        ) {
          const filteredExtrinsics = {};

          Object.entries(filteredBlock.extrinsics).forEach(
            ([_, extrinsics]) => {
              if (extrinsics?.method.pallet) {
                const palletName = extrinsics.method.pallet;
                if (palletName !== "paraInherent") {
                  filteredExtrinsics[palletName] = extrinsics;
                }
              }
            },
          );

          filteredBlock.extrinsics = filteredExtrinsics;
        }

        return filteredBlock;
      });

      // Store the filtered data for pagination
      window.blockData.blocks = filteredData;
      window.blockData.currentPage = 0; // Reset to first page

      // Create styles for pagination
      ensureBlockStyles();

      // Render the current page
      renderBlockPage();
    } else {
      dataDiv.innerHTML = "<p>No blocks found for this address.</p>";
    }
  } catch (error) {
    console.error("Error fetching blocks:", error);
    showError("blocks", error.message);
  }
}

// Function to ensure block styles are applied
function ensureBlockStyles() {
  // Only add styles if they don't already exist
  if (!document.getElementById("block-styles")) {
    const style = document.createElement("style");
    style.id = "block-styles";
    style.textContent = `
            .nested-table {
                margin-bottom: 0 !important;
            }
            .nested-table th, .nested-table td {
                padding: 0.3em 0.5em !important;
            }
            .nested-table .nested-table {
                font-size: 0.95em;
                border: 1px solid #eee;
            }
            .block-number {
                font-weight: bold;
                width: 120px;
            }
            .break-word {
                word-break: break-all;
            }
            .block-row td {
                vertical-align: top;
            }
            .address-highlight {
                background-color: #ffff00; /* Fluorescent yellow */
                font-weight: bold;
                padding: 1px 2px;
            }
            .pagination {
                display: flex;
                justify-content: center;
                margin-top: 20px;
                margin-bottom: 10px;
            }
            .pagination button {
                margin: 0 5px;
            }
            .pagination-info {
                margin: 0 10px;
                line-height: 2.25em;
            }
        `;
    document.head.appendChild(style);
  }
}

// Function to render the current block page
function renderBlockPage() {
  const { blocks, currentPage, pageSize } = window.blockData;
  const dataDiv = document.getElementById("blocks-data");
  const searchInput = document.getElementById("search-address");

  const startIndex = currentPage * pageSize;
  const endIndex = Math.min(startIndex + pageSize, blocks.length);
  const pageBlocks = blocks.slice(startIndex, endIndex);
  const totalPages = Math.ceil(blocks.length / pageSize);

  let html = "";

  // Add pagination controls
  html += '<div class="pagination">';
  html += `<button class="button" ${currentPage === 0 ? "disabled" : ""} onclick="prevBlockPage()"><i class="fas fa-chevron-left"></i> Previous</button>`;
  html += `<span class="pagination-info">Page ${currentPage + 1} of ${totalPages} (${blocks.length} blocks)</span>`;
  html += `<button class="button" ${currentPage >= totalPages - 1 ? "disabled" : ""} onclick="nextBlockPage()">Next <i class="fas fa-chevron-right"></i></button>`;
  html += "</div>";

  // Add block table
  html += '<table class="table is-fullwidth is-hoverable result-table">';
  html += "<thead><tr><th>Block</th><th>Content</th></tr></thead>";
  html += "<tbody>";

  pageBlocks.forEach((block) => {
    // First filter out any null or empty properties
    const blockContent = {};
    Object.entries(block).forEach(([key, value]) => {
      if (
        value !== null &&
        value !== undefined &&
        !(Array.isArray(value) && value.length === 0) &&
        !(typeof value === "object" && Object.keys(value).length === 0)
      ) {
        blockContent[key] = value;
      }
    });

    const blockNumber = block.block_number || block.number || "N/A";
    const blockKeys = Object.keys(blockContent).sort((a, b) => {
      // Always put block_number and hash first
      if (a === "block_number" || a === "number") return -1;
      if (b === "block_number" || b === "number") return 1;
      if (a === "hash") return -1;
      if (b === "hash") return 1;
      return a.localeCompare(b);
    });

    if (blockKeys.length > 0) {
      // Main block row
      html += `<tr class="block-row">`;
      html += `<td class="block-number">${blockNumber}</td>`;

      // Content column with nested table
      html += "<td>";
      html +=
        '<table class="table is-fullwidth is-bordered is-narrow nested-table">';
      html += "<tbody>";

      blockKeys.forEach((key) => {
        // Skip block number since it's already in the first column
        if (key === "block_number" || key === "number") return;

        const value = blockContent[key];
        html += `<tr><td width="150">${key}</td><td>${renderBlockContent(value, searchInput.value.trim())}</td></tr>`;
      });

      html += "</tbody></table>";
      html += "</td>";
      html += "</tr>";
    }
  });

  html += "</tbody></table>";

  // Repeat pagination controls at bottom
  html += '<div class="pagination">';
  html += `<button class="button" ${currentPage === 0 ? "disabled" : ""} onclick="prevBlockPage()"><i class="fas fa-chevron-left"></i> Previous</button>`;
  html += `<span class="pagination-info">Page ${currentPage + 1} of ${totalPages} (${blocks.length} blocks)</span>`;
  html += `<button class="button" ${currentPage >= totalPages - 1 ? "disabled" : ""} onclick="nextBlockPage()">Next <i class="fas fa-chevron-right"></i></button>`;
  html += "</div>";

  dataDiv.innerHTML = html;
}

// Functions for pagination navigation
window.prevBlockPage = function () {
  if (window.blockData.currentPage > 0) {
    window.blockData.currentPage--;
    renderBlockPage();
  }
};

window.nextBlockPage = function () {
  const totalPages = Math.ceil(
    window.blockData.blocks.length / window.blockData.pageSize,
  );
  if (window.blockData.currentPage < totalPages - 1) {
    window.blockData.currentPage++;
    renderBlockPage();
  }
};

// Export functions
export { fetchBlocks };
