package runners

import (
	"context"
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/kaleido-io/kaleido-fabric-go/fabric"
	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
)

type SDKRunner struct {
	user          string
	chaincode     string
	count         int
	workers       int
	initChaincode bool
	channel       *kaleido.Channel
	sdk           *fabsdk.FabricSDK
}

func NewSDKRunner(user, chaincode string, count, workers int, initChaincode bool) *SDKRunner {
	return &SDKRunner{
		user:          user,
		chaincode:     chaincode,
		count:         count,
		workers:       workers,
		initChaincode: initChaincode,
	}
}

func (s *SDKRunner) Exec() error {
	fmt.Println("Using the Fabric SDK for transaction submission")
	err := s.init()
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
	_, err := s.channel.InitChaincode(s.chaincode)
	if err != nil {
		fmt.Printf("Failed to initialize chaincode: %s\n", err)
		return err
	}

	return nil
}

func (s *SDKRunner) runTransactions() error {
	ctx := context.Background()
	done := make(chan interface{})
	sequence := 0
	completed := 0
	for ; sequence < s.workers; sequence++ {
		worker := NewWorker(ctx, s.chaincode, sequence, done, nil, s.channel)
		worker.Start(sequence)
	}

	for {
		<-done
		completed++
		if sequence < s.count {
			worker := NewWorker(ctx, s.chaincode, sequence, done, nil, s.channel)
			worker.Start(sequence)
			sequence++
		} else {
			if completed == s.count {
				break
			}
		}
	}

	fmt.Print("\nAll transactions submitted\n")

	return nil
}

func (s *SDKRunner) init() error {
	network := kaleido.NewNetwork()
	network.Initialize()
	config, err := fabric.BuildConfig(network)
	if err != nil {
		fmt.Printf("Failed to generate network configuration for the SDK: %s\n", err)
		return err
	}

	sdk1, err := newSDK(config)
	if err != nil {
		return err
	}
	defer sdk1.Close()

	wallet := kaleido.NewWallet(s.user, *network, sdk1)
	err = wallet.InitIdentity()
	if err != nil || wallet.Signer == nil {
		fmt.Printf("Failed to initiate wallet: %v\n", err)
		return err
	}

	fabric.AddTlsConfig(config, wallet.Signer)

	sdk2, err := newSDK(config)
	s.sdk = sdk2
	if err != nil {
		return err
	}

	s.channel = kaleido.NewChannel(network.TargetChannel.Name, sdk2)
	err = s.channel.Connect(wallet.Signer.Identifier())
	if err != nil {
		fmt.Printf("Failed to connect to channel: %s\n", err)
		return err
	}

	return nil
}

func newSDK(config map[string]interface{}) (*fabsdk.FabricSDK, error) {
	configProvider, err := fabric.NewConfigProvider(config)
	if err != nil {
		fmt.Printf("Failed to create config provider from config map: %s\n", err)
		return nil, err
	}

	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		fmt.Printf("Failed to instantiate an SDK: %s\n", err)
		return nil, err
	}
	return sdk, nil
}
