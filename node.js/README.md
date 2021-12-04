# node.js Sample App For Kaleido-base Fabric networks
Sample application to use a Kaleido based Hyperledger Fabric blockchain to initialize and invoke chaincode transactions.

## Instructions
The application is logically made up of 3 parts:
- build the common connections profile (CCP) based on Kaleido's network topology
- register and enroll the identity to submit the transaction
- submit the transaction

For the first part, the program uses [Kaleido's platform API](https://api.kaleido.io/platform.html) to gether the information about the orderers and peers in the network, as well as channels and memberships, in order to build the CCP to be used by the SDK.

Note that the discovery service built-in to the peer can also be used with Kaleido Fabric networks. However, because of the cloud networking involved in the Kaleido orderer and peer nodes, the same node is represented by two hostnames, one used in the internal peer-to-peer virtual network and another used to interact with the outside world. Because of this, the discovery service will return both of these entries as if they were separate nodes. This results in the SDKs to emit error logs that look like the following:

```
2021-06-10T16:34:49.531Z - error: [ServiceEndpoint]: Error: Failed to connect before the deadline on Committer- name: zzeas018vt.zzozz0yjym.kaleido.network:40040, url:grpcs://zzeas018vt.zzozz0yjym.kaleido.network:40040, connected:false, connectAttempted:true
2021-06-10T16:34:49.532Z - error: [ServiceEndpoint]: waitForReady - Failed to connect to remote gRPC server zzeas018vt.zzozz0yjym.kaleido.network:40040 url:grpcs://zzeas018vt.zzozz0yjym.kaleido.network:40040 timeout:3000
2021-06-10T16:34:49.533Z - error: [DiscoveryService]: _buildOrderer[default-channel] - Unable to connect to the discovered orderer zzeas018vt.zzozz0yjym.kaleido.network:40040 due to Error: Failed to connect before the deadline on Committer- name: zzeas018vt.zzozz0yjym.kaleido.network:40040, url:grpcs://zzeas018vt.zzozz0yjym.kaleido.network:40040, connected:false, connectAttempted:true
```

This should be harmless as the SDKs are able to filter out the entries that are not connected, so the external hostnames returned by the discovery service should still be sufficient to fulfill the transaction submission.

Alternatively, you can disable using the discovery service when setting up the Gateway instance, and instead rely on the CCP object for the orderer and peer endpoints.

The sample app supports both modes.

### Prerequisite
Required environment variables:
- `APIKEY`: Kaleido API key created in your account's profile

Optional environment variables:
- `KALEIDO_URL`: the root URL for the Kaleido API endpoints. Default is `https://console.kaleido.io/api/v1`
- `USER_ID`: name of the user to register and enroll with the Fabric CA service, to be used to submit transactions. Default is `user01`
- `CCNAME`: name of the chaincode to invoke. Default is `asset_transfer`
- `USE_DISCOVERY`: whether to use the discovery service or not. Default is `false`.

Follow the instructions in [the documentation](https://docs.kaleido.io/kaleido-platform/protocol/fabric/fabric/) to create a channel and deploy a chaincode in your Kaleido Fabric network. The name of the Apps project will be used as the chaincode name (value of the `CCNAME` environment variable).

**IMPORTANT**: The Fabric node.js SDK version 2.2, which is the latest version as of June 2021, does not support initialization of chaincodes. When deploying the chaincode, you much uncheck the option _Require initialization before invocation_ in the UI panel.

### Run the program
Run the program and follow the prompts to select the target consortium, environment, membership and channel.

```
npm i
node app.js
```
