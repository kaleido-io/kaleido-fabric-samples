# Golang Sample App For Kaleido-base Fabric networks

Sample application to use a Kaleido based Hyperledger Fabric blockchain to initialize and invoke chaincode transactions.

## Overview

The application is logically made up of 3 parts:

- build the common connections profile (CCP) based on Kaleido's network topology (if _not_ using FabConnect)
- register and enroll the identity to submit the transaction
- submit the transaction

For the first part, the program uses [Kaleido's platform API](https://api.kaleido.io/platform.html) to gether the information about the orderers and peers in the network, as well as channels and memberships, in order to build the CCP to be used by the SDK. However, if you use FabConnect to submit the transactions, this part will be skipped.

## Run Against FabConnect

You can use this client to driver transactions against a FabConnect instance:

```
export USE_FABCONNECT=true
export FABCONNECT_URL=https://appCredId:appCredPassword@u0c0ljzevw-u0wwnrkcne-connect.u0-ws.kaleido.io
export USER_ID=signer1
export CHANNEL_ID=mychannel
export CCNAME=asset_transfer
export EVENT_BATCH_SIZE=250
export TX_COUNT=500
export WORKERS=500
./kfg
```

## Run With a Common Connection Profile

You can use this client against a Fabric network directly, by providing a Common Connection Profile YAML file.

```
export CCP=/Users/myname/Documents/perf-test/ccp.yaml
export USER_ID=signer1
export CHANNEL_ID=mychannel
export CCNAME=asset_transfer
export EVENT_BATCH_SIZE=250
export TX_COUNT=500
export WORKERS=500
./kfg
```

## Run Against A Kaleido Network

You can use this client against a Kaleido based Fabric environment.

- `APIKEY`: This is required - the Kaleido API key created in your account's profile
- `KALEIDO_URL`: (optional) the root URL for the Kaleido API endpoints. Default is `https://console.kaleido.io/api/v1`
- `CONSORTIUM`: (optional) Kaleido consortium ID. If not supplied, you will be prompted for the consortium you are a member of
- `ENVIRONMENT`: (optional) Kaleido environment ID. If not supplied, you will be prompted for the environment
- `SUBMITTER`: (optional) Kaleido membership ID to use when registering and enrolling the transaction signing identity. If not supplied, you will be prompted for the membership if you own more than one in the consortium

## Common

- `USER_ID`: (optional) name of the user to register and enroll with the Fabric CA service, to be used to submit transactions. Default is `user01`
- `CCNAME`: (optional) name of the chaincode to invoke. Default is `asset_transfer`
- `INIT_CC`: (optional) whether this run is to initialize the chaincode (if the chaincode has been deployed with the `--init-required` parameter). Default is `false`
- `TX_COUNT`: (optional) number of total transactions to submit. Default is `1`.
- `WORKERS`: (optional) number of concurrent workers to submit transactions. If the `TX_COUNT` is larger than the `WORKERS`, a worker must have already completed the task before a new worker is kicked off, until all the transactions are processed. Default is `1`. Max is `50`.

Follow the instructions in [the documentation](https://docs.kaleido.io/kaleido-platform/protocol/fabric/fabric/) to create a channel and deploy a chaincode in your Kaleido Fabric network. The name of the Apps project will be used as the chaincode name (value of the `CCNAME` environment variable).
