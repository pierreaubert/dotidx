// import { web3Accounts, web3Enable, web3FromAddress } from './bundle-polkadot-extension-dapp.js';

export let selectedPolkadotAccount = null;
export let allPolkadotAccount = null;

// Function to check for Polkadot extension and connect
async function connectPolkadotJs() {
    const connectButton = document.getElementById('polkadot-connect-button');

    // Check if the extension is installed (window.injectedWeb3 should exist)
    // Wait a bit for the extension to inject the object
    await new Promise((resolve) => setTimeout(resolve, 500));

    try {
        // Request permission (web3Enable returns enabled extensions)
        const extensions = await polkadotExtensionDapp.web3Enable('DIX');
        if (extensions.length === 0) {
            alert('Permission denied for Polkadot{.js} extension.');
            return;
        }

        // Get accounts (web3Accounts returns accounts from enabled extensions)
        const allAccounts = await polkadotExtensionDapp.web3Accounts();
        if (allAccounts.length === 0) {
            alert('No accounts found in Polkadot{.js} extension.');
            connectButton.textContent = 'No Accounts Found';
            connectButton.disabled = true;
            return;
        }

        // Use the first account for simplicity
        selectedPolkadotAccount = allAccounts[0];
        allPolkadotAccount = allAccounts;
        console.log('Connected Polkadot Account:', selectedPolkadotAccount);

        // Update UI
	let accountDisplay = document.getElementById('addresses');
	if (accountDisplay) {
	    let html = '';
	    allAccounts.forEach( (account) => {
		const shortAddress = `${account.address.substring(0, 6)}...${account.address.substring(account.address.length - 6)}`;
		const text = `${shortAddress} (${selectedPolkadotAccount.meta.name})`;
		html += `<option value="${account.address}">${text}</option>`;
	    });
	    html += '<option value="">Add manually</option>';
	    accountDisplay.innerHTML = html;
            accountDisplay.style.display = 'inline';
	}

	let newUrl = window.location.search;
	newUrl += '&address=' + selectedPolkadotAccount.address;
	window.history.pushState({}, '', newUrl);

    } catch (error) {
        console.error('Error connecting to Polkadot{.js} extension:', error);
        alert(`Error connecting: ${error.message}`);
    }
}

export async function initPJS() {
    const connectButton = document.getElementById('polkadot-connect-button');
    if (connectButton) {
        connectButton.addEventListener('click', connectPolkadotJs);
    } else {
        console.error('Polkadot connect button not found after nav update.');
    }
}

