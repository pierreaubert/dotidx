function processBalance(b) {
    let r = parseFloat(b);
    r = r / 10 / 1000 / 1000 / 1000;
    return Math.round(r * 100) / 100;
}

export async function getAccountAt(address, blockid) {
    const balanceUrl = '/proxy/accounts/' + address + '/balance-info?at=' + blockid;
    const response = await fetch(balanceUrl, { mode: 'cors' });
    if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
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
