import { fetchBlocks } from "./blocks.js";
import { fetchBalances } from "./balances.js";
import { fetchStaking } from "./staking.js";
import { fetchCompletionRate, fetchMonthlyStats } from "./stats.js";

document.addEventListener("DOMContentLoaded", () => {
  initApp();
});

function initApp() {

  const searchInput = document.getElementById("search-address");
  const actionButton = document.getElementById("action-button");
  const tabsContainer = document.querySelector(".tabs ul");
  const tabContents = document.querySelectorAll(".tab-content");

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
    if (tabId === "balances-tab") {
      actionButton.textContent = "Search Balances";
      fetchBalances();
    } else if (tabId === "blocks-tab") {
      actionButton.textContent = "Search Blocks";
      fetchBlocks();
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
    tabs.forEach((tab) => {
      const tabDataAttr = tab.getAttribute("data-tab");
      if (tabDataAttr === tabId) {
        tab.classList.add("is-active");
      } else {
        tab.classList.remove("is-active");
      }
    });

    // Update tab content visibility
    const tabContents = document.querySelectorAll(".tab-content");
    tabContents.forEach((content) => {
      if (content.id === tabId) {
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

      if (tabElement.tagName === "A") {
        tabElement = tabElement.parentElement; // If clicked on the A tag, get its parent LI
      }

      if (tabElement.tagName === "LI") {
        const tabId = tabElement.getAttribute("data-tab");
        if (tabId) {
          setActiveTab(tabId);
        } else {
          console.error("Tab element missing data-tab attribute:", tabElement);
        }
      }
    });
  }

  // Call the initialization function
  init();
}
