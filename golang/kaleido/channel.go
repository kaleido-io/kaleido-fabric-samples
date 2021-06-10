package kaleido

import (
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

type Channel struct {
	ChannelID string
	client    *channel.Client
	sdk       *fabsdk.FabricSDK
}

func NewChannel(channelId string, sdk *fabsdk.FabricSDK) *Channel {
	return &Channel{
		ChannelID: channelId,
		sdk:       sdk,
	}
}

func (c *Channel) Connect(signer *msp.IdentityIdentifier) error {
	channelContext := c.sdk.ChannelContext(c.ChannelID, fabsdk.WithUser(signer.ID), fabsdk.WithOrg(signer.MSPID))
	channelClient, err := channel.New(channelContext)
	if err != nil {
		return fmt.Errorf("Failed to create channel client. %s", err)
	}
	c.client = channelClient
	return nil
}

func (c *Channel) InitChaincode(chaincodeId string) error {
	return c.invokeChaincode(chaincodeId)
}

func (c *Channel) ExecChaincode(chaincodeId, assetId string) error {
	return c.invokeChaincode(chaincodeId, assetId)
}

func (c *Channel) invokeChaincode(chaincodeId string, assetName ...string) error {
	if len(assetName) == 0 {
		_, err := c.client.Execute(
			channel.Request{ChaincodeID: chaincodeId, Fcn: "InitLedger", IsInit: true},
			channel.WithRetry(retry.DefaultChannelOpts),
		)
		if err != nil {
			return fmt.Errorf("Failed to send transaction to initialize the chaincode. %s", err)
		}
	} else {
		_, err := c.client.Execute(
			channel.Request{ChaincodeID: chaincodeId, Fcn: "CreateAsset", Args: [][]byte{[]byte(assetName[0]), []byte("yellow"), []byte("10"), []byte("Tom"), []byte("1300")}},
			channel.WithRetry(retry.DefaultChannelOpts),
		)
		if err != nil {
			return fmt.Errorf("Failed to send transaction to invoke the chaincode. %s", err)
		}
	}
	return nil
}
