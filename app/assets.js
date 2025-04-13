// balances and staking are very similar
// this file contains functions shared by both

import { updateModals, updateIcons, updateFooter, updateNav, updateSearchAssets } from './components.js';
import { initPJS } from './wallet.js';

export function getAddress() {
    const e = document.getElementById('addresses');
    const value = e.options[e.selectedIndex].value;
    // TODO: add atest to check for Polkadot/ETH address
    if (value) {
        return value.trim();
    }
    return null;
}

export function addAddress(address) {
    let searchInput = document.getElementById('addresses');
    const pos = [...searchInput.options].map((v) => v.value).indexOf(address);
    if (pos === -1) {
        let option = document.createElement('option');
        option.value = address;
        option.text = address.slice(0, 6) + ' ... ' + address.slice(-6);
        option.selected = true;
        searchInput.add(option);
    } else {
        searchInput.options[pos].selected = true;
    }
}

function updateUrl(target, name) {
    let newUrl = name + '?';
    const address = getAddress();

    if (address && address.length > 10) {
        newUrl += `&address=${encodeURIComponent(address)}`;
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
        addAddress(address);
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

    document.getElementById('search-from').value = '';
    document.getElementById('search-to').value = '';

    if (searchInput.value.trim()) {
        window.history.pushState({}, '', `?address=${encodeURIComponent(searchInput.value.trim())}`);
    } else {
        window.history.pushState({}, '', window.location.pathname);
    }
}

function updateAddresses(_event) {
    const e = document.getElementById('addresses');
    const value = e.options[e.selectedIndex].value;
    if (!value || value === '') {
        const modal = document.getElementById('modal-add-address');
        modal.classList.add('is-active');
    } else {
        document.getElementById('action-button').click();
    }
}

function getAddressFromModal(_event) {
    const modal = document.getElementById('add-address');
    addAddress(modal.value);
    document.getElementById('modal-add-address').classList.remove('is-active');
}

// target: name of the div
// name: name of the address in url (balances, staking ...)
// fetchIt: a function that will be called back when clicking
export async function initAddresses(target, name, fetchIt) {
    await updateIcons();
    await updateNav();
    await updateFooter();
    await updateModals();
    await updateSearchAssets(target);
    await initPJS();

    const actionButton = document.getElementById('action-button');
    actionButton.addEventListener('click', () => {
        const url = updateUrl('addresses', name);
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

    document.getElementById('addresses').addEventListener('change', (e) => {
        updateAddresses(e);
    });
    document.getElementById('addresses').addEventListener('click', (e) => {
        updateAddresses(e);
    });

    document.getElementById('add-address').addEventListener('change', (e) => {
        getAddressFromModal(e);
    });

    document.getElementById('add-address-add-button').addEventListener('click', (e) => {
        getAddressFromModal(e);
    });

    document.getElementById('add-address-cancel-button').addEventListener('click', (e) => {
        getAddressFromModal(e);
    });
}
