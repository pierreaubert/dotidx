import { isHex, hexToU8a } from '@polkadot/util';
import { decodeAddress, encodeAddress } from '@polkadot/util-crypto';

export function isValidSubstrateAddress(address) {
    try {
        encodeAddress(isHex(address) ? hexToU8a(address) : decodeAddress(address));
        return true;
    } catch (error) {
        console.log('invalid address: ' + address + ' ' + error);
        return false;
    }
}

export function substrate2polkadot(address) {
    const pubKey = decodeAddress(address);
    return encodeAddress(pubKey, 0);
}

export function substrate2kusama(address) {
    const pubKey = decodeAddress(address);
    return encodeAddress(pubKey, 2);
}
