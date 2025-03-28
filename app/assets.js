// balances and staking are very similar
// this file contains functions shared by both

import { updateFooter, updateNav, updateSearchAssets } from './components.js';

function updateUrl(target, name) {
    const searchInput = document.getElementById(target);
    let newUrl = name;

    const address = searchInput.value.trim();
    if (address) {
        newUrl += `?address=${address}`;
    }

    const count = document.getElementById('search-count').value.trim();
    if (count) {
        newUrl += `&count=${encodeURIComponent(count)}`;
    }
    const fromDate = document.getElementById('search-from').value;
    if (fromDate) {
        const fromDateTime = new Date(fromDate);
        newUrl += `&from=${encodeURIComponent(fromDateTime.toISOString())}`;
    }

    const toDate = document.getElementById('search-to').value;
    if (toDate) {
        const toDateTime = new Date(toDate);
        newUrl += `&to=${encodeURIComponent(toDateTime.toISOString())}`;
    }

    window.history.pushState({}, '', newUrl);
    return newUrl;
}

function updateFromUrl() {
    const urlParams = new URLSearchParams(window.location.search);

    const address = urlParams.get('address');
    if (address) {
        document.getElementById('search-address').value = address;
    }

    const count = urlParams.get('count');
    if (count) {
        document.getElementById('search-count').value = count;
    }

    const fromParam = urlParams.get('from');
    if (fromParam) {
        const fromDate = new Date(fromParam);
        document.getElementById('search-from').value = fromDate.toISOString().slice(0, 16);
    }

    const toParam = urlParams.get('to');
    if (toParam) {
        const toDate = new Date(toParam);
        document.getElementById('search-to').value = toDate.toISOString().slice(0, 16);
    }

    if (address) {
        const actionButton = document.getElementById('action-button');
        actionButton.click();
    }
}

function clearFilters() {
    const searchInput = document.getElementById('search-address');

    document.getElementById('search-count').value = '20';
    document.getElementById('search-from').value = '';
    document.getElementById('search-to').value = '';

    if (searchInput.value.trim()) {
        window.history.pushState({}, '', `?address=${encodeURIComponent(searchInput.value.trim())}`);
    } else {
        window.history.pushState({}, '', window.location.pathname);
    }
}

// target: name of the div
// name: name of the address in url (balances, staking ...)
// fetchIt: a function that will be called back when clicking
export async function initAddresses(target, name, fetchIt) {
    await updateNav();
    await updateFooter();
    await updateSearchAssets(target);

    const actionButton = document.getElementById('action-button');
    actionButton.addEventListener('click', () => {
        const url = updateUrl('search-address', name);
        fetchIt(url);
    });

    const clearFiltersButton = document.getElementById('clear-filters');
    clearFiltersButton.addEventListener('click', () => {
        clearFilters();
        actionButton.click();
    });

    const applyFiltersButton = document.getElementById('apply-filters');
    applyFiltersButton.addEventListener('click', () => {
        actionButton.click();
    });

    updateFromUrl();
}
