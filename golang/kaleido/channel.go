package kaleido

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	log "github.com/sirupsen/logrus"
)

type Channel struct {
	ChannelID string
	client    *channel.Client
	sdk       *fabsdk.FabricSDK
	Start     time.Time
}

func NewChannel(channelId string, sdk *fabsdk.FabricSDK) *Channel {
	return &Channel{
		ChannelID: channelId,
		sdk:       sdk,
	}
}

func (c *Channel) Connect(signer *msp.IdentityIdentifier, orgId string) error {
	channelContext := c.sdk.ChannelContext(c.ChannelID, fabsdk.WithUser(signer.ID), fabsdk.WithOrg(orgId))
	channelClient, err := channel.New(channelContext)
	if err != nil {
		return fmt.Errorf("failed to create channel client. %s", err)
	}
	c.client = channelClient
	return nil
}

func (c *Channel) InitChaincode(channel, chaincodeId string) (string, error) {
	return c.invokeChaincode(chaincodeId)
}

func (c *Channel) ExecChaincode(channel, chaincodeId, assetId string) (string, error) {
	return c.invokeChaincode(chaincodeId, assetId)
}

func (c *Channel) SubscribeEvents(chaincodeId string, done chan string, total int) (fab.Registration, error) {
	reg, notifier, err := c.client.RegisterChaincodeEvent(chaincodeId, "AssetCreated")
	if err != nil {
		return nil, fmt.Errorf("failed to register chaincode event. %s", err)
	}

	go func() {
		eventsReceived := 0
		for event := range notifier {
			log.Infof("Received chaincode event with tx ID: %s", event.TxID)
			eventsReceived++
			if eventsReceived >= total {
				break
			}
		}
		done <- "done"
	}()

	return reg, nil
}

func (c *Channel) UnsubscribeEvents(reg fab.Registration) {
	c.client.UnregisterChaincodeEvent(reg)
}

func (c *Channel) invokeChaincode(chaincodeId string, assetName ...string) (string, error) {
	var resp channel.Response
	var err error
	if len(assetName) == 0 {
		resp, err = c.client.Execute(
			channel.Request{ChaincodeID: chaincodeId, Fcn: "InitLedger", IsInit: true},
			channel.WithRetry(retry.DefaultChannelOpts),
		)
		if err != nil {
			return "", fmt.Errorf("failed to send transaction to initialize the chaincode. %s", err)
		}
	} else {
		resp, err = c.client.Execute(
			channel.Request{ChaincodeID: chaincodeId, Fcn: "CreateAsset", Args: [][]byte{[]byte(assetName[0]), []byte("yellow"), []byte("10"), []byte("Tom"), []byte("1300")}},
			channel.WithRetry(retry.DefaultChannelOpts),
		)
		if err != nil {
			return "", fmt.Errorf("failed to send transaction to invoke the chaincode. %s", err)
		}
	}
	return string(resp.TransactionID), nil
}
