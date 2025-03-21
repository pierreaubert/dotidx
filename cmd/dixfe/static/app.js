// app.js - Main JavaScript file for DotIDX Dashboard
// This file imports and coordinates all functionality from the module files

// Import functionality from module files
import { fetchBlocks } from "./blocks.js";
import { fetchBalances } from "./balances.js";
import { fetchStaking } from "./staking.js";
import { fetchCompletionRate, fetchMonthlyStats } from "./stats.js";

// Wait for DOM to be fully loaded before initializing
document.addEventListener("DOMContentLoaded", () => {
  console.log("DOM loaded, initializing application");
  initApp();
});

// Main application initialization function
function initApp() {
  // Get common elements used throughout the app
  const searchInput = document.getElementById("search-address");
  const actionButton = document.getElementById("action-button");
  const tabsContainer = document.querySelector(".tabs ul");
  const tabContents = document.querySelectorAll(".tab-content");
  let activeTab = "";

  console.log("Elements found:", {
    searchInput: !!searchInput,
    actionButton: !!actionButton,
    tabsContainer: !!tabsContainer,
    tabContents: tabContents.length,
  });

  // Process URL parameters
  const urlParams = new URLSearchParams(window.location.search);
  const addressParam = urlParams.get("address");
  const countParam = urlParams.get("count");
  const fromParam = urlParams.get("from");
  const toParam = urlParams.get("to");

  // Populate form fields from URL parameters if present
  if (addressParam) {
    searchInput.value = addressParam;
  }
  if (countParam) {
      document.getElementById("search-count").value = countParam;
  }
  if (fromParam) {
    // Convert from ISO format to local datetime-local format
    const fromDate = new Date(fromParam);
    document.getElementById("search-from").value = fromDate
      .toISOString()
      .slice(0, 16);
  }
  if (toParam) {
    // Convert from ISO format to local datetime-local format
    const toDate = new Date(toParam);
    document.getElementById("search-to").value = toDate
      .toISOString()
      .slice(0, 16);
  }

  // Function to update URL with current parameters based on active tab
  function updateUrl() {
    const address = searchInput.value.trim();
    if (!address) return; // Don't update URL if no address is entered

    const count = document.getElementById("search-count").value.trim();
    const fromDate = document.getElementById("search-from").value;
    const toDate = document.getElementById("search-to").value;

    let newUrl = `?address=${encodeURIComponent(address)}`;

    if (count) {
      newUrl += `&count=${encodeURIComponent(count)}`;
    }

    if (fromDate) {
      const fromDateTime = new Date(fromDate);
      newUrl += `&from=${encodeURIComponent(fromDateTime.toISOString())}`;
    }

    if (toDate) {
      const toDateTime = new Date(toDate);
      newUrl += `&to=${encodeURIComponent(toDateTime.toISOString())}`;
    }

    // Update URL without reloading the page
    window.history.pushState({}, "", newUrl);
  }

  // Function to clear filters and reset to defaults
  function clearFilters() {
    document.getElementById("search-count").value = "";
    document.getElementById("search-from").value = "";
    document.getElementById("search-to").value = "";

    // Also clear the URL parameters
    if (searchInput.value.trim()) {
      // Only keep the address parameter
      window.history.pushState(
        {},
        "",
        `?address=${encodeURIComponent(searchInput.value.trim())}`,
      );
    } else {
      // Clear all parameters
      window.history.pushState({}, "", window.location.pathname);
    }
  }

  // Function to set the active tab
  function setActiveTab(tabId) {
    console.log("Setting active tab to:", tabId);
    activeTab = tabId;

    // Debug DOM elements for each tab to identify issues
    if (tabId === "balances-tab") {
      const balanceResult = document.getElementById("search-result");
      const balanceData = document.getElementById("search-data");
      const balanceGraph = document.getElementById("search-graph");
      console.log("Balance tab elements:", {
        balanceResult: !!balanceResult,
        balanceData: !!balanceData,
        balanceGraph: !!balanceGraph,
      });
    } else if (tabId === "blocks-tab") {
      const blocksResult = document.getElementById("blocks-result");
      const blocksData = document.getElementById("blocks-data");
      console.log("Blocks tab elements:", {
        blocksResult: !!blocksResult,
        blocksData: !!blocksData,
      });
    } else if (tabId === "stats-tab") {
      const completionResult = document.getElementById("completion-result");
      const monthlyResult = document.getElementById("monthly-result");
      console.log("Stats tab elements:", {
        completionResult: !!completionResult,
        monthlyResult: !!monthlyResult,
      });
    }

    // Update the action button behavior based on active tab
    console.log("Executing " + tabId + " fetchBalances() function");
    if (tabId === "balances-tab") {
      actionButton.textContent = "Search Balances";
      fetchBalances();
    } else if (tabId === "blocks-tab") {
      actionButton.textContent = "Search Blocks";
      fetchBlocks();
      console.log("Set action button for blocks tab");
    } else if (tabId === "stats-tab") {
      actionButton.textContent = "Refresh Stats";
      fetchCompletionRate();
      fetchMonthlyStats();
    } else if (tabId === "staking-tab") {
      actionButton.textContent = "Search Staking";
      fetchStaking();
    }
    updateUrl();

    // Update the active tab styling
    const tabs = document.querySelectorAll(".tabs li");
    console.log("Found", tabs.length, "tab elements");
    tabs.forEach((tab) => {
      const tabDataAttr = tab.getAttribute("data-tab");
      console.log(
        "Tab element:",
        tabDataAttr,
        "comparing with",
        tabId,
        "isMatch:",
        tabDataAttr === tabId,
      );
      if (tabDataAttr === tabId) {
        tab.classList.add("is-active");
      } else {
        tab.classList.remove("is-active");
      }
    });

    // Update tab content visibility
    const tabContents = document.querySelectorAll(".tab-content");
    console.log("Found", tabContents.length, "tab content elements");
    tabContents.forEach((content) => {
      console.log(
        "Tab content ID:",
        content.id,
        "comparing with",
        tabId,
        "isMatch:",
        content.id === tabId,
      );
      if (content.id === tabId) {
        console.log("Activating content:", content.id);
        content.classList.add("is-active");
        content.classList.remove("is-hidden");
      } else {
        content.classList.remove("is-active");
        content.classList.add("is-hidden");
      }
    });
  }

  // Initialize the app
  function init() {
    console.log("Initializing application...");

    // Verify tabs and contents are available
    if (!tabsContainer) {
      console.error("No tabs container found!");
    }

    if (tabContents.length === 0) {
      console.error("No tab contents found!");
    }

    // Set initial active tab to match HTML
    const initialActiveTab = document.querySelector(".tabs li.is-active");
    if (initialActiveTab) {
      const initialTabId = initialActiveTab.getAttribute("data-tab");
      console.log("Setting initial active tab to:", initialTabId);
      activeTab = initialTabId;
      setActiveTab(initialTabId);
    } else {
      console.warn("No active tab found in HTML, defaulting to blocks-tab");
      setActiveTab("blocks-tab");
    }

    // Allow Enter key to trigger search
    searchInput.addEventListener("keyup", (event) => {
      if (event.key === "Enter") {
        actionButton.click();
      }
    });

    // If address is provided in URL, trigger search automatically
    if (addressParam) {
      actionButton.click();
    }

    // Set up clear filters button if it exists
    const clearFiltersButton = document.getElementById("clear-filters");
    if (clearFiltersButton) {
      clearFiltersButton.addEventListener("click", clearFilters);
    }

    // Initialize filters section if it exists
    const filtersToggle = document.getElementById("filters-toggle");
    if (filtersToggle) {
      filtersToggle.addEventListener("click", function () {
        const filtersSection = document.getElementById("filters-section");
        if (filtersSection) {
          filtersSection.classList.toggle("is-hidden");
          this.classList.toggle("is-active");
        }
      });
    }

    // Add event listener for tab clicks
    tabsContainer.addEventListener("click", (event) => {
      // Find the nearest LI element - either the target itself or its parent
      let tabElement = event.target;

      console.log(
        "Tab click event captured on:",
        tabElement.tagName,
        tabElement,
      );

      // Handle click on A tag inside LI
      if (tabElement.tagName === "A") {
        tabElement = tabElement.parentElement; // If clicked on the A tag, get its parent LI
        console.log("Click on A tag, parent is:", tabElement.tagName);
      }

      // Only proceed if we have an LI element
      if (tabElement.tagName === "LI") {
        const tabId = tabElement.getAttribute("data-tab");
        console.log("Tab element found, data-tab attribute:", tabId);

        if (tabId) {
          setActiveTab(tabId);
        } else {
          console.error("Tab element missing data-tab attribute:", tabElement);
        }
      }
    });
  }

  // Helper function to show error messages
  function showError(section, message) {
    const resultDiv = document.getElementById(`${section}-result`);
    const dataDiv = document.getElementById(`${section}-data`);

    if (resultDiv && dataDiv) {
      resultDiv.classList.remove("is-hidden");
      dataDiv.innerHTML = `<div class="notification is-danger">${message}</div>`;
    }
  }

  // Call the initialization function
  init();
}
