'use strict';

const prompt = require('prompt-sync')();
const KaleidoClient = require('./lib/kaleido');
const { Gateway } = require('fabric-network');
const Client = require('fabric-common');

const chaincodeName = process.env.CCNAME || 'asset_transfer';
const userId = process.env.USER_ID || 'user01';
const useDiscovery = process.env.USE_DISCOVERY || 'false';

main();

async function main() {
  const kclient = new KaleidoClient(userId);
  await kclient.init();

  const gateway = new Gateway();
  try {
    // setup the gateway instance
    // The user will now be able to create connections to the fabric network and be able to
    // submit transactions and query. All transactions submitted by this gateway will be
    // signed by this user using the credentials stored in the wallet.
    await gateway.connect(kclient.config, {
      wallet: kclient.wallet.wallet,
      identity: userId,
      discovery: { enabled: useDiscovery === 'true', asLocalhost: false }
    });

    // Build a network instance based on the channel where the smart contract is deployed
    const network = await gateway.getNetwork(kclient.channel.name);
    const contract = network.getContract(chaincodeName);

    const isInit = prompt('Calling "InitLedger" (y/n)? ');
    let fcn, args;
    if (isInit === 'y') {
      fcn = 'InitLedger';
      args = [];
    } else {
      fcn = 'CreateAsset';
      const assetId = `asset-${Math.floor(Math.random() * 1000000)}`;
      console.log(`Generating a random asset ID to use to create a new asset: ${assetId}`);
      args = [assetId, "yellow", "5", "Tom", "1300"];
    }
    // Initialize a set of asset data on the channel using the chaincode 'InitLedger' function.
    console.log(`\n--> Submitting Transaction. fcn: ${fcn}, args: ${args}`);
    await contract.submitTransaction(fcn, ...args);
    console.log('*** Result: committed');

  } catch (error) {
		console.error(`******** FAILED to run the application: ${error.stack ? error.stack : error}`);
	} finally {
    // Disconnect from the gateway when the application is closing
    // This will close all connections to the network
    gateway.disconnect();
  }
}