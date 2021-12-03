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
      clientTlsIdentity: userId,
      discovery: { enabled: useDiscovery === 'true', asLocalhost: false }
    });

    // Build a network instance based on the channel where the smart contract is deployed
    const network = await gateway.getNetwork(kclient.channel.name);
    const contract = network.getContract(chaincodeName);

    const isInit = prompt('Calling "InitLedger" (y/n)? ');
    if (isInit === 'y') {
      // Initialize ledger with demo assets
      const fcn = 'InitLedger';
      console.log(`--> Submitting Transaction. fcn: ${fcn}`);
      await contract.submitTransaction(fcn);
      console.log('*** Result: committed');
    } else {
      // Add an asset with a random id to the channel using the 'CreateAsset' chaincode function
      let fcn = 'CreateAsset';
      const assetId = `asset-${Math.floor(Math.random() * 1000000)}`;
      console.log(`Adding new asset with following random asset ID to the ledger: ${assetId}`);
      let args = [assetId, "yellow", "5", "Benny", "53000"];

      console.log(`--> Submitting Transaction. fcn: ${fcn}, args: ${args}`);
      await contract.submitTransaction(fcn, ...args);
      console.log('*** Result: committed');

      // Read just created asset from the channel using the 'ReadAsset' chaincode function
      fcn = 'ReadAsset';
      console.log(`Reading asset with ID: ${assetId}`);
      args = [assetId];

      console.log(`--> Evaluating Transaction. fcn: ${fcn}, args: ${args}`);
      const blockchainResponse = await contract.evaluateTransaction(fcn, ...args);
      console.log(`*** Result: ${blockchainResponse}`);
    }
  } catch (error) {
		console.error(`******** FAILED to run the application: ${error.stack ? error.stack : error}`);
	} finally {
    // Disconnect from the gateway when the application is closing
    // This will close all connections to the network
    gateway.disconnect();
  }
}
