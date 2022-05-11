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
	rand.Seed(time.Now().UTC().UnixNano())

	username := os.Getenv("USER_ID")
	if username == "" {
		username = "user1"
	}

	ccname := os.Getenv("CCNAME")
	if ccname == "" {
		ccname = "asset_transfer"
	}

	var err error
	var channel *kaleido.Channel
	var fabconnectClient *kaleido.FabconnectClient
	useFabconnect := os.Getenv("USE_FABCONNECT")

	if useFabconnect == "true" {
		fabconnectUrl := os.Getenv("FABCONNECT_URL")
		fabconnectClient = kaleido.NewFabconnectClient(fabconnectUrl, username)
		fmt.Println("Using Fabconnect for transaction submission")

		err := fabconnectClient.EnsureIdentity()
		if err != nil {
			fmt.Printf("Failed to ensure Fabconnect identity. %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Using Fabconnect identity: %s\n", username)
	} else {
		fmt.Println("Using the Fabric SDK for transaction submission")
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

		channel = kaleido.NewChannel(network.TargetChannel.Name, sdk2)
		err = channel.Connect(wallet.Signer.Identifier())
		if err != nil {
			fmt.Printf("Failed to connect to channel: %s\n", err)
			os.Exit(1)
		}
	}

	initChaincode := os.Getenv("INIT_CC")
	if initChaincode == "" {
		initChaincode = "false"
	} else {
		initChaincode = strings.ToLower(initChaincode)
	}

	if initChaincode == "true" {
		if useFabconnect == "true" {
			err = fabconnectClient.InitChaincode(ccname)
		} else {
			err = channel.InitChaincode(ccname)
		}
		if err != nil {
			fmt.Printf("Failed to initialize chaincode: %s\n", err)
		} else if useFabconnect == "true" {
			var batchWg sync.WaitGroup
			monitorFabconnectBatchReceipts(&batchWg, fabconnectClient, 0)
			batchWg.Wait()
			fabconnectClient.PrintFinalReport(1)
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
		if count > 50 {
			fmt.Println("Error: TX_COUNT cannot exceed 50")
			os.Exit(1)
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
					fmt.Printf("=> Batch %d: Send transaction %d of %d (%s)\n", i+1, idx+1, count, assetId)
					if useFabconnect == "true" {
						err = fabconnectClient.ExecChaincode(false, i, ccname, assetId)
					} else {
						err = channel.ExecChaincode(ccname, assetId)
					}
					if err != nil {
						fmt.Printf("=> Batch %d: Failed to send transaction %d (%s). %s\n", i+1, idx+1, assetId, err)
					}
				}(j)
			}
			wg.Wait()

			fmt.Printf("\nCompleted batch %d of %d\n\n", i+1, batches)

			if i < (batches-1) && useFabconnect != "true" {
				fmt.Println("Sleeping for 30 seconds before the next batch")
				time.Sleep(30 * time.Second)
			}
		}

		if useFabconnect == "true" {
			var batchWg sync.WaitGroup
			for i := 0; i < batches; i++ {
				monitorFabconnectBatchReceipts(&batchWg, fabconnectClient, i)
			}
			batchWg.Wait()
			fabconnectClient.PrintFinalReport(batches)
		}

		fmt.Printf("\nAll Done!\n")
	}
}

func monitorFabconnectBatchReceipts(batchWg *sync.WaitGroup, fabconnectClient *kaleido.FabconnectClient, batch int) {
	batchWg.Add(1)
	go func(idx int) {
		defer batchWg.Done()
		fmt.Printf("=> Batch %d: Start Monitoring for transaction receipts\n", idx+1)
		fabconnectClient.MonitorBatch(idx)
	}(batch)
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
