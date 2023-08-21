/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const DEFAULT_BASE_URL = "/api";
const baseUrlInput = document.getElementById('baseUrlInput')
const networkInput = document.getElementById('networkInput')
const networkError = document.getElementById('networkError')
let baseUrl;
let networks = {};
let networkToAccount = {};

baseUrlInput.addEventListener('change', async function() {
    baseUrl = baseUrlInput.value;
    await updateNetworks();
});

async function updateNetworks() {
    networkError.parentElement.hidden = true
    networkError.innerText = '';
    const selected = networkInput.value;
    const resp = await fetch(baseUrl, {method: 'GET'}).catch(e => {
        networkError.parentElement.hidden = false;
        networkError.innerText = e;
    });
    if (!resp) {
        return
    }
    networks = {};
    const networkArr = await resp.json();
    let networkInputInnerHTML = '';
    networkArr.map(e => {
        networks[e.network] = e.type;
        networkInputInnerHTML += `<option value="${e.network}">${e.network}</option>`;
    });
    networkInput.innerHTML = networkInputInnerHTML;
    if (selected) {
        networkInput.value = selected;
    }
}

async function ensureNetwork(network) {
    await updateNetworks();
    const networkType = networks[network];
    if (!networkType) {
        throw new Error(`not supported network:${network}`);
    }
    return networkType;
}


const ethereumProviderInput = document.getElementById('ethereumProviderInput')
const ethereumProviderError = document.getElementById('ethereumProviderError')
const META_MASK = "MetaMask";
const HANA_WALLET = "HanaWallet";
const hanaWalletEthereumProvider = hanaWalletEthereum();
const metaMaskEthereumProvider = metaMaskEthereum();
let ethereumProvider;
function hanaWalletEthereum() {
    if (window.hanaWallet && window.hanaWallet.available) {
        return window.hanaWallet.ethereum;
    } else {
        throw new Error("hana wallet not available")
    }
}

function metaMaskEthereum() {
    // console.log(MetaMaskSDK);
    const opt = {
        forceInjectProvider: typeof window.ethereum === 'undefined',
    }
    const MMSDK = new MetaMaskSDK.MetaMaskSDK(opt);
    return MMSDK.getProvider() // You can also access via window.ethereum
}

async function updateEthereumProvider() {
    ethereumProviderError.parentElement.hidden = true;
    ethereumProviderError.innerText = '';
    const selected = ethereumProviderInput.value;
    if (ethereumProvider === undefined) {
        switch (selected) {
            case META_MASK:
                ethereumProvider = metaMaskEthereumProvider;
                break;
            case HANA_WALLET:
                ethereumProvider = hanaWalletEthereumProvider;
                break;
        }
    } else {
        switch (selected) {
            case META_MASK:
                if (!ethereumProvider.isMetaMask || ethereumProvider.isHanaWallet) {
                    ethereumProvider = metaMaskEthereumProvider;
                }
                break;
            case HANA_WALLET:
                if (!ethereumProvider.isHanaWallet) {
                    ethereumProvider = hanaWalletEthereumProvider;
                }
                break;
        }
    }
    if (ethereumProvider.isConnected()) {
        return new Promise(resolve => resolve());
    }
    return ethereumProvider.request({method: 'eth_requestAccounts'}).catch(e => {
        ethereumProviderError.parentElement.hidden = false
        ethereumProviderError.innerText = e
    });
}

ethereumProviderInput.addEventListener('change', async function() {
    await updateEthereumProvider(ethereumProviderInput.value);
});

function updateAccount(network, account) {
    networkToAccount[network] = account;
    console.log(`network:${network} account:${account}`);
}

async function ensureAccount(network) {
    console.log(`ensureAccount network:${network}`);
    const networkType = await ensureNetwork(network);
    switch (networkType) {
        case "eth":
        case "eth2":
        case "bsc":
            const accounts = await ethereumProvider.request({method: 'eth_requestAccounts'});
            return new Promise(resolve => {
                const account = accounts[0];
                updateAccount(network, account);
                resolve(account);
            });
        case "icon":
            const iconRequestAddress = new CustomEvent('ICONEX_RELAY_REQUEST', {
                detail: {type: 'REQUEST_ADDRESS'}
            });
            const r = window.dispatchEvent(iconRequestAddress);
            console.log(`ensureAccount dispatchEvent ${r}`);
            return new Promise(resolve => {
                const handler = evt => {
                    console.log(evt.detail);
                    if (evt.detail.type !== 'RESPONSE_ADDRESS') {
                        throw new Error(`not expected event:${evt.detail.type}`);
                    }
                    window.removeEventListener('ICONEX_RELAY_RESPONSE', handler);
                    const account = evt.detail.payload;
                    updateAccount(network, account);
                    resolve(account)
                    console.log("ensureAccount after resolve");
                }
                window.addEventListener('ICONEX_RELAY_RESPONSE', handler);
            })
        default:
            throw new Error(`not supported network type:${networkType}`);
    }
}

function b64ToHex(b64) {
    return '0x' + atob(b64).split('').map((m) => ('0' + m.charCodeAt(0).toString(16)).slice(-2)).join('');
}

function removeHexPrefix(hex) {
    if (hex.substring(0, 2) === "0x") {
        hex = hex.substring(2);
    }
    return hex
}

function hexToB64(hex) {
    hex = removeHexPrefix(hex);
    return btoa(hex.match(/.{1,2}/g).map((byte) => String.fromCodePoint(parseInt(byte, 16))).join(''))
}

const compactSigMagicOffset = 27;

function recoverFlagToCompatible(signature) {
    let flag = parseInt(signature.substring(signature.length - 2), 16);
    if (flag >= compactSigMagicOffset) {
        flag = flag - compactSigMagicOffset;
        return signature.substring(0, signature.length - 2) + flag.toString(16).padStart(2, '0');
    }
    return signature;
}

function postData(data) {
    return {
        method: 'POST',
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
    }
}

async function invoke(service, method, params, account) {
    const network = networkInput.value;
    const url = `${baseUrl}/${network}/${service}/${method}`
    if (!account) {
        account = await ensureAccount(network);
    }
    let data = {
        "options": {
            "From": account,
        },
        "params": params
    }
    let resp = await fetch(url, postData(data));
    if (resp.status !== 200) {
        const errResp = await resp.json();
        if (errResp.code !== 1005) {
            return new Promise(resolve => resolve(errResp));
        }
        const hexData = b64ToHex(errResp.data.data);
        console.log(`data:${errResp.data.data}, ${hexData}`)
        const hexSignature = await sign(network, hexData, account);
        data.options = errResp.data.options;
        data.options.Signature = hexToB64(hexSignature);
        resp = await fetch(url, postData(data));
    }
    return resp.json()
}

async function sign(network, hexData, account) {
    console.log(`network:${network} hexData:${hexData} account:${account}`);
    const networkType = networks[network];
    switch (networkType) {
        case "eth":
        case "eth2":
        case "bsc":
            const hexSignature = await ethereumProvider.request({
                method: 'eth_sign',
                params: [account, hexData],
            });
            return new Promise(resolve => {
                console.log(`network:${network} type:${networkType} signature: ${hexSignature}`);
                resolve(recoverFlagToCompatible(hexSignature));
            });
        case "icon":
            const iconRequestSigning = new CustomEvent('ICONEX_RELAY_REQUEST', {
                detail: {type: 'REQUEST_SIGNING', payload: {from: account, hash: removeHexPrefix(hexData)}}
            });
            const r = window.dispatchEvent(iconRequestSigning);
            console.log(`sign dispatchEvent ${r}`);
            return new Promise((resolve, reject) => {
                const handler = evt => {
                    console.log(evt.detail);
                    if (evt.detail.type === 'CANCEL_SIGNING') {
                        reject(new Error(`user canceled signing request`));
                    }
                    if (evt.detail.type !== 'RESPONSE_SIGNING') {
                        reject(new Error(`not expected event:${evt.detail.type}`));
                    }
                    window.removeEventListener('ICONEX_RELAY_RESPONSE', handler);
                    const signature = evt.detail.payload;
                    const hexSignature = b64ToHex(signature);
                    console.log(`icon signature: ${signature} hexSignature: ${hexSignature}`);
                    resolve(recoverFlagToCompatible(hexSignature));
                    console.log("sign after resolve");
                }
                window.addEventListener('ICONEX_RELAY_RESPONSE', handler);
            });
        default:
            throw new Error(`not support network type:${networkType}`);
    }
}

async function call(service, method, params) {
    const network = networkInput.value;
    const url = `${baseUrl}/${network}/${service}/${method}`
    let data = {
        "options": {},
        "params": params
    }

    let resp = await fetch(url+"?"+Qs.stringify(data), {
        method: 'GET',
    });
    return resp.json();
}

async function initialize() {
    baseUrl = DEFAULT_BASE_URL;
    baseUrlInput.value = baseUrl;
    await updateNetworks();

    let ethereumProviderInputInnerHTML = `<option value="${HANA_WALLET}">${HANA_WALLET}</option>`;
    ethereumProviderInputInnerHTML += `<option value="${META_MASK}">${META_MASK}</option>`;
    ethereumProviderInput.innerHTML = ethereumProviderInputInnerHTML;
    await updateEthereumProvider();
}

window.addEventListener('load', initialize);
