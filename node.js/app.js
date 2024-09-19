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

    
    if (process.env.LIST_ASSETS) {
      // The current chaincode sample has no method for retrieving single assets
      // This branch allows for a one-off listing of all assets from the contract
      console.log(`Reading all assets`);
      let fcn = 'GetAllAssets';
      let args = [];
      console.log(`--> Evaluating Transaction. fcn: ${fcn}, args: ${args}`);
      const blockchainResponse = await contract.evaluateTransaction(fcn, ...args);
      console.log(`*** Result: ${blockchainResponse}`);
    } else {
      let isInit;
      if (process.env.IS_INIT) {
        isInit = process.env.IS_INIT;
      } else {
        isInit = prompt('Calling "InitLedger" (y/n)? ');
      }
      let NUM_ITERATIONS = process.env.NUM_ITERATIONS ? parseInt(process.env.NUM_ITERATIONS) : 1000;
      for (let i = 0; i < NUM_ITERATIONS; i++) {
        let fcn, args, assetId;
        if (i === 0 && isInit === 'y') {
          // Initialize a set of asset data on the channel using the chaincode 'InitLedger' function.
          fcn = 'InitLedger';
          args = [];
          isInit = 'n'
        } else {
          // Create an asset on the channel using the chaincode 'CreateAsset' function
          fcn = 'CreateAsset';
          assetId = `asset-${Math.floor(Math.random() * 1000000)}`;
          console.log(`Generating a random asset ID to use to create a new asset: ${assetId}`);
          args = [assetId, "yellow", "5", "Tom", "1300"];
        }
        console.log(`\n--> Submitting Transaction. fcn: ${fcn}, args: ${args}`);
        await contract.submitTransaction(fcn, ...args);
        console.log('*** Result: committed');
      }
    }
  } catch (error) {
		console.error(`******** FAILED to run the application: ${error.stack ? error.stack : error}`);
	} finally {
    // Disconnect from the gateway when the application is closing
    // This will close all connections to the network
    gateway.disconnect();
  }
}
