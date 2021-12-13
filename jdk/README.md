# Java Sample App For Kaleido-base Fabric networks

Sample application to use a Kaleido based Hyperledger Fabric blockchain to initialize and invoke chaincode transactions.

## Overview

The application is logically made up of 3 parts:

- build the common connections profile (CCP) based on Kaleido's network topology
- register and enroll the identity to submit the transaction
- submit the transaction and query for the result

For the first part, the program uses [Kaleido's platform API](https://api.kaleido.io/platform.html) to gether the information about the orderers and peers in the network, as well as channels and memberships, in order to build the CCP to be used by the SDK.

## Prerequisite

Required environment variables:

- `APIKEY`: Kaleido API key created in your account's profile

Optional environment variables:

- `KALEIDO_URL`: the root URL for the Kaleido API endpoints. Default is `https://console.kaleido.io/api/v1`
- `USER_ID`: name of the user to register and enroll with the Fabric CA service, to be used to submit transactions. Default is `user01`
- `CCNAME`: name of the chaincode to invoke. Default is `asset_transfer`

Follow the instructions in [the documentation](https://docs.kaleido.io/kaleido-platform/protocol/fabric/fabric/) to create a channel and deploy a chaincode in your Kaleido Fabric network. The name of the Apps project will be used as the chaincode name (value of the `CCNAME` environment variable).

## Run the program

```
./gradlew run --console=plain
```
