function processBalance(b) {
    let r = parseFloat(b);
    r = r / 10 / 1000 / 1000 / 1000;
    return Math.round(r * 100) / 100;
}

export const default_balance = {
    symbol: 'N/A',
    free: 0.0,
    frozen: 0.0,
    reserved: 0.0,
};

export async function getAccountAt(relay, chain, address, blockid) {
    let balanceUrl = `/proxy/${relay}/${chain}/accounts/${address}/balance-info`;
    if (blockid != '') {
        balanceUrl += `?at=${blockid}`;
    }
    const response = await fetch(balanceUrl, { mode: 'cors' }).catch((err) => {
        // network error
        console.warn('got ' + err + ' when calling ' + balanceUrl);
        return default_balance;
    });
    if (!response.ok) {
        console.warn('got ' + response.status + ' when calling ' + balanceUrl);
        return default_balance;
    }
    const textRaw = await response.text();
    const result = await JSON.parse(textRaw);

    return {
        symbol: result.tokenSymbol,
        free: processBalance(result.free),
        frozen: processBalance(result.frozen),
        reserved: processBalance(result.reserved),
    };
}
