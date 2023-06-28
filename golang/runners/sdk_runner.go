package runners

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	coremsp "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/kaleido-io/kaleido-fabric-go/fabric"
	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
	log "github.com/sirupsen/logrus"
)

type SDKRunner struct {
	user          string
	channel       string
	chaincode     string
	count         int
	workers       int
	initChaincode bool
	channelClient *kaleido.Channel
	sdk           *fabsdk.FabricSDK
}

func NewSDKRunner(user, channel, chaincode string, count, workers int, initChaincode bool) *SDKRunner {
	return &SDKRunner{
		user:          user,
		channel:       channel,
		chaincode:     chaincode,
		count:         count,
		workers:       workers,
		initChaincode: initChaincode,
	}
}

func (s *SDKRunner) Exec() error {
	log.Info("Using the Fabric SDK for transaction submission")
	err := s.init(s.channel)
	if err != nil {
		return err
	}
	defer s.sdk.Close()

	if s.initChaincode {
		err = s.runInitChaincode()
	} else {
		err = s.runTransactions()
	}
	if err != nil {
		return err
	}

	return nil
}

func (s *SDKRunner) runInitChaincode() error {
	txId, err := s.channelClient.InitChaincode(s.channel, s.chaincode)
	if err != nil {
		log.Errorf("Failed to initialize chaincode: %s", err)
		return err
	}

	log.Infof("Chaincode initialized. TxId: %s", txId)
	return nil
}

func (s *SDKRunner) runTransactions() error {
	ctx := context.Background()
	// assign each worker the transaction count
	done, workers := allocateWorkers(ctx, s.channel, s.chaincode, s.count, s.workers, s.channelClient)

	// subscribe to events
	reg, err := s.channelClient.SubscribeEvents(s.chaincode, done, s.count)
	if err != nil {
		log.Errorf("Failed to subscribe to events: %s", err)
		return err
	}

	s.channelClient.Start = time.Now()

	// start workers
	for _, w := range workers {
		w.Start()
	}

	<-done

	defer s.channelClient.UnsubscribeEvents(reg)

	printFinalReport(s.count, s.workers, 1, s.channelClient.Start)

	return nil
}

func (s *SDKRunner) init(channel string) error {
	var signer *coremsp.IdentityIdentifier
	var orgId string
	// if a CCP YAML is provided, use it to initialize the SDK
	ccpFile := os.Getenv("CCP")
	if ccpFile != "" {
		si, orgID, err := s.initWithCCP(ccpFile)
		if err != nil {
			log.Errorf("Failed to instantiate an SDK: %s", err)
			return err
		}
		signer = si.Identifier()
		orgId = orgID
	} else {
		selectedChannel, signerId, orgID, err := s.initWithKaleido()
		if err != nil {
			log.Errorf("Failed to instantiate an SDK: %s", err)
			return err
		}
		signer = signerId
		orgId = orgID
		channel = selectedChannel
	}

	s.channelClient = kaleido.NewChannel(channel, s.sdk)
	err := s.channelClient.Connect(signer, orgId)
	if err != nil {
		log.Errorf("Failed to connect to channel: %s", err)
		return err
	}

	return nil
}

func (s *SDKRunner) initWithCCP(ccpFile string) (coremsp.SigningIdentity, string, error) {
	configProvider := config.FromFile(ccpFile)
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		log.Errorf("Failed to instantiate an SDK: %s", err)
		return nil, "", err
	}
	s.sdk = sdk
	ctxProvider := sdk.Context()
	backends, err := configProvider()
	configBackend := backends[0]
	orgId, ok := configBackend.Lookup("client.organization")
	if !ok {
		log.Errorf("Failed to get organization from config provider")
		return nil, "", err
	}
	caConfigs, ok := configBackend.Lookup(fmt.Sprintf("organizations.%s.certificateAuthorities", orgId.(string)))
	if !ok {
		log.Errorf("Failed to get certificateAuthorities from config provider")
		return nil, "", err
	}
	caNames := caConfigs.([]interface{})
	if len(caNames) == 0 {
		log.Errorf("certificateAuthorities has no entries")
		return nil, "", err
	}
	mspClient, err := msp.New(ctxProvider, msp.WithCAInstance(caNames[0].(string)))
	if err != nil {
		log.Errorf("Failed to create Fabric CA client. %v", err)
		return nil, "", err
	}
	si, err := mspClient.GetSigningIdentity(s.user)
	if err != nil {
		log.Errorf("Failed to get signing identity: %v.", err)
		return nil, "", err
	}
	return si, orgId.(string), nil
}

func (s *SDKRunner) initWithKaleido() (string, *coremsp.IdentityIdentifier, string, error) {
	network := kaleido.NewNetwork()
	network.Initialize()
	config, err := fabric.BuildConfig(network)
	if err != nil {
		log.Errorf("Failed to generate network configuration for the SDK: %s", err)
		return "", nil, "", err
	}

	sdk1, err := newSDK(config)
	if err != nil {
		return "", nil, "", err
	}
	defer sdk1.Close()

	wallet := kaleido.NewWallet(s.user, *network, sdk1)
	err = wallet.InitIdentity()
	if err != nil || wallet.Signer == nil {
		log.Errorf("Failed to initiate wallet: %v", err)
		return "", nil, "", err
	}

	fabric.AddTlsConfig(config, wallet.Signer)

	sdk2, err := newSDK(config)
	s.sdk = sdk2
	if err != nil {
		return "", nil, "", err
	}
	return network.TargetChannel.Name, wallet.Signer.Identifier(), network.MyMembership.ID, nil
}

func newSDK(config map[string]interface{}) (*fabsdk.FabricSDK, error) {
	configProvider, err := fabric.NewConfigProvider(config)
	if err != nil {
		log.Errorf("Failed to create config provider from config map: %s", err)
		return nil, err
	}

	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		log.Errorf("Failed to instantiate an SDK: %s", err)
		return nil, err
	}
	return sdk, nil
}
