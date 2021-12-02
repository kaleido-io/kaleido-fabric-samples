package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/kaleido-io/kaleido-fabric-go/fabric"
	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
)

func main() {
	username := os.Getenv("USER_ID")
	if username == "" {
		username = "user1"
	}

	ccname := os.Getenv("CCNAME")
	if ccname == "" {
		ccname = "asset_transfer"
	}

	network := kaleido.NewNetwork()
	network.Initialize()
	config, err := fabric.BuildConfig(network)
	if err != nil {
		fmt.Printf("Failed to generate network configuration for the SDK: %s\n", err)
		os.Exit(1)
	}

	sdk1 := newSDK(config)
	defer sdk1.Close()

	wallet := kaleido.NewWallet(username, *network, sdk1)
	err = wallet.InitIdentity()
	if err != nil || wallet.Signer == nil {
		fmt.Printf("Failed to initiate wallet: %v\n", err)
		os.Exit(1)
	}

	fabric.AddTlsConfig(config, wallet.Signer)

	sdk2 := newSDK(config)
	defer sdk2.Close()

	channel := kaleido.NewChannel("default-channel", sdk2)
	err = channel.Connect(wallet.Signer.Identifier())
	if err != nil {
		fmt.Printf("Failed to connect to channel: %s\n", err)
		os.Exit(1)
	}

	initChaincode := os.Getenv("INIT_CC")
	if initChaincode == "" {
		initChaincode = "false"
	} else {
		initChaincode = strings.ToLower(initChaincode)
	}

	if initChaincode == "true" {
		err = channel.InitChaincode(ccname)
		if err != nil {
			fmt.Printf("Failed to initialize chaincode: %s\n", err)
		}
	} else {
		var count int
		countStr := os.Getenv("TX_COUNT")
		if countStr != "" {
			count, err = strconv.Atoi(countStr)
			if err != nil {
				fmt.Printf("Failed to convert %s to integer", countStr)
				os.Exit(1)
			}
		} else {
			count = 1
		}
		var batches int
		batchStr := os.Getenv("BATCHES")
		if batchStr != "" {
			batches, err = strconv.Atoi(batchStr)
			if err != nil {
				fmt.Printf("Failed to convert %s to integer", batchStr)
				os.Exit(1)
			}
		} else {
			batches = 1
		}

		for i := 0; i < batches; i++ {

			var wg sync.WaitGroup
			for j := 0; j < count; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					assetId := fmt.Sprintf("asset%d", rand.Int())
					fmt.Printf("=> Start sending transactions %d of %d (%s)\n", idx+1, count, assetId)
					err = channel.ExecChaincode(ccname, assetId)
					if err != nil {
						fmt.Printf("=> Failed to send transaction %d (%s). %s\n", idx+1, assetId, err)
					} else {
						fmt.Printf("=> Completed sending transactions %d of %d (%s)\n", idx+1, count, assetId)
					}
				}(j)
			}
			wg.Wait()

			if i < (batches - 1) {
				fmt.Printf("\nCompleted batch %d of %d, sleeping for 30 seconds before the next batch\n\n", i+1, batches)
				time.Sleep(30 * time.Second)
			} else {
				fmt.Printf("\nCompleted batch %d of %d\n\n", i+1, batches)
			}
		}

		fmt.Printf("\nAll Done!\n")
	}
}

func newSDK(config map[string]interface{}) *fabsdk.FabricSDK {
	configProvider, err := fabric.NewConfigProvider(config)
	if err != nil {
		fmt.Printf("Failed to create config provider from config map: %s\n", err)
		os.Exit(1)
	}

	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		fmt.Printf("Failed to instantiate an SDK: %s\n", err)
		os.Exit(1)
	}
	return sdk
}
