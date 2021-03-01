# kaleido-fabric-samples
Sample application to use a Kaleido based Hyperledger Fabric blockchain to initialize and invoke chaincode transactions.

## Instructions
The current version only supports centralized environments, where all peers are owned by the same organization (but can be distributed under multiple memberships). Future enhancements may support decentralized environments, where peer endpoint information must be provided from out-of-band communications.

### Prerequisite
Required environment variables:
- `APIKEY`: Kaleido API key
- `PATH`: the system path environment variable must include the folder containing the Fabric `peer` command

Optional environment variables:
- `KALEIDO_URL`: the root URL for the Kaleido API endpoints. Default is `https://console.kaleido.io`
- `USER_ID`: name of the user to register and enroll with the Fabric CA service, to be used to submit transactions. Default is `user01`
- `CCNAME`: name of the chaincode to invoke. Default is `asset_transfer`.

Build a chaincode binary using a golang based chaincode implementation and deploy it using the Kaleido Smart Contract management. The name of the contract project will be used as the chaincode name (value of the `CCNAME` environment variable).

### Run the program
Run the program and follow the prompts to select the target consortium, environment, membership and channel.

```
npm i
node app.js
```
