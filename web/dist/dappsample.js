const DAPP_SAMPLE_SERVICE = 'dappsample'
const toInput = document.getElementById('toInput')
const dataInput = document.getElementById('dataInput')
const rollbackInput = document.getElementById('rollbackInput')
const getBTPAddressButton = document.getElementById('getBTPAddressButton')
const getBTPAddressResult = document.getElementById('getBTPAddressResult')
const sendMessageButton = document.getElementById('sendMessageButton')
const sendMessageResult = document.getElementById('sendMessageResult')

getBTPAddressButton.addEventListener('click', async function (event) {
    event.preventDefault();
    try {
        getBTPAddressButton.disabled = true;
        const resp = await call(DAPP_SAMPLE_SERVICE, 'getBTPAddress',  {});
        getBTPAddressResult.innerHTML = JSON.stringify(resp);
    } catch (err) {
        console.error(err);
        getBTPAddressResult.innerHTML = `Error: ${err.message}`;
    }
    getBTPAddressButton.disabled = false;
});

sendMessageButton.addEventListener('click', async function (event) {
    event.preventDefault();
    try {
        sendMessageButton.disabled = true;
        const resp = await invoke(DAPP_SAMPLE_SERVICE, 'sendMessage',  {
            "_to" : toInput.value,
            "_data" : dataInput.value,
            "_rollback": rollbackInput.value,
        });
        sendMessageResult.innerHTML = JSON.stringify(resp);
    } catch (err) {
        console.error(err);
        sendMessageResult.innerHTML = `Error: ${err.message}`;
    }
    sendMessageButton.disabled = false;
});

const getResultButton = document.getElementById('getResultButton')
getResultButton.addEventListener('click', async function (event) {
    event.preventDefault();
    try {
        getResultButton.disabled = true;
        const network = networkInput.value;
        const txId = JSON.parse(sendMessageResult.innerHTML);
        const getResultUrl = `${baseUrl}/${network}/result/${txId}`
        const resp = await fetch(getResultUrl, {method:'GET'});
        txResult.innerHTML = await resp.text();
    } catch (err) {
        console.error(err);
        txResult.innerHTML = `Error: ${err.message}`;
    }
    getResultButton.disabled = false;
});
