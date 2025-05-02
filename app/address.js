export function isValidSubstrateAddress(address) {
    try {
        polkadotUtilCrypto.encodeAddress(
            polkadotUtil.isHex(address) ? polkadotUtil.hexToU8a(address) : polkadotUtilCrypto.decodeAddress(address)
        );
        return true;
    } catch (error) {
        console.log('invalid address: ' + address + ' ' + error);
        return false;
    }
}

export function substrate2polkadot(address) {
    const pubKey = polkadotUtilCrypto.decodeAddress(address);
    return polkadotUtilCrypto.encodeAddress(pubKey, 0);
}

export function substrate2kusama(address) {
    const pubKey = polkadotUtilCrypto.decodeAddress(address);
    return polkadotUtilCrypto.encodeAddress(pubKey, 2);
}
