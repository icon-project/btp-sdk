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
let services = {};
let networks = {};
let networkToAccount = {};

baseUrlInput.addEventListener('change', async function() {
    baseUrl = baseUrlInput.value;
    await updateServices();
});

async function updateServices() {
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
    services = {};
    networks = {};
    const serviceArr = await resp.json();
    let networkInputInnerHTML = '';
    serviceArr.map(e => {
        services[e.name] = e
        Object.keys(e.networks).map(n => {
            networks[n] = e.networks[n];
        });
    });
    Object.keys(networks).map(n => {
        networkInputInnerHTML += `<option value="${n}">${n}</option>`;
    })
    networkInput.innerHTML = networkInputInnerHTML;
    if (selected) {
        networkInput.value = selected;
    }
}

async function ensureNetwork(network) {
    await updateServices();
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
let ethereumProviders = {};
let ethereumProvider;
function hanaWalletEthereum() {
    if (window.hanaWallet && window.hanaWallet.available) {
        return window.hanaWallet.ethereum;
    } else {
        throw new Error("hana wallet not available")
    }
}

async function metaMaskEthereum() {
    // console.log(MetaMaskSDK);
    const opt = {
        forceInjectProvider: typeof window.ethereum === 'undefined',
    }
    const MMSDK = new MetaMaskSDK.MetaMaskSDK(opt);
    await MMSDK.init();
    return MMSDK.getProvider() // You can also access via window.ethereum
}

async function updateEthereumProvider() {
    ethereumProviderError.parentElement.hidden = true;
    ethereumProviderError.innerText = '';
    const selected = ethereumProviderInput.value;
    if (ethereumProvider === undefined) {
        ethereumProviders[META_MASK] = await metaMaskEthereum();
        ethereumProviders[HANA_WALLET] = hanaWalletEthereum();
    }
    ethereumProvider = ethereumProviders[selected];
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
            return new Promise((resolve, reject) => {
                const handler = evt => {
                    console.log(evt.detail);
                    window.removeEventListener('ICONEX_RELAY_RESPONSE', handler);
                    if (evt.detail.type !== 'RESPONSE_ADDRESS') {
                        reject(new Error(`not expected event:${evt.detail.type}`));
                        return
                    }
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
    const url = `${baseUrl}/${service}/${method}`
    if (!account) {
        account = await ensureAccount(network);
    }
    let data = {
        "network": network,
        "options": {
            "from": account,
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
        data.options.signature = hexToB64(hexSignature);
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
                    window.removeEventListener('ICONEX_RELAY_RESPONSE', handler);
                    if (evt.detail.type === 'CANCEL_SIGNING') {
                        reject(new Error(`user canceled signing request`));
                        return
                    }
                    if (evt.detail.type !== 'RESPONSE_SIGNING') {
                        reject(new Error(`not expected event:${evt.detail.type}`));
                        return
                    }
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
    const url = `${baseUrl}/${service}/${method}`
    let data = {
        "network": network,
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
    await updateServices();

    let ethereumProviderInputInnerHTML = `<option value="${HANA_WALLET}">${HANA_WALLET}</option>`;
    ethereumProviderInputInnerHTML += `<option value="${META_MASK}">${META_MASK}</option>`;
    ethereumProviderInput.innerHTML = ethereumProviderInputInnerHTML;
    await updateEthereumProvider();
}

window.addEventListener('load', initialize);
