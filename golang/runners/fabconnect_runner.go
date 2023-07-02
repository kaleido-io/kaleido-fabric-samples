package runners

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
	log "github.com/sirupsen/logrus"
)

type FabconnectRunner struct {
	user          string
	channel       string
	chaincode     string
	count         int
	workers       int
	initChaincode bool
	client        *kaleido.FabconnectClient
}

func NewFabconnectRunner(user, channel, chaincode string, count, workers int, initChaincode bool) *FabconnectRunner {
	return &FabconnectRunner{
		user:          user,
		channel:       channel,
		chaincode:     chaincode,
		count:         count,
		workers:       workers,
		initChaincode: initChaincode,
	}
}

func (f *FabconnectRunner) Exec() error {
	log.Info("Using Fabconnect for transaction submission")

	fabconnectUrl := os.Getenv("FABCONNECT_URL")
	client, err := kaleido.NewFabconnectClient(fabconnectUrl, f.user)
	if err != nil {
		log.Errorf("Failed to create Fabconnect client. %v", err)
		return err
	}
	f.client = client

	err = f.client.EnsureIdentity()
	if err != nil {
		log.Errorf("Failed to ensure Fabconnect identity. %v", err)
		return err
	}
	log.Infof("Using Fabconnect identity: %s", f.user)

	if f.initChaincode {
		err = f.runInitChaincode()
	} else {
		err = f.runTransactions()
	}
	if err != nil {
		return err
	}

	return nil
}

func (f *FabconnectRunner) runInitChaincode() error {
	receiptId, err := f.client.InitChaincode(f.channel, f.chaincode)
	if err != nil {
		log.Errorf("Failed to initialize chaincode: %s", err)
		return err
	}

	receipt, err := f.client.GetReceipt(receiptId)
	if err != nil {
		log.Errorf("Failed to query receipt %s", receiptId)
		return err
	}

	if receipt.Headers.Type == "TransactionSuccess" {
		log.Info("Chaincode init successful")
	}

	return nil
}

func (f *FabconnectRunner) runTransactions() error {
	ctx := context.Background()
	// assign each worker the transaction count
	eventAssetIdsChan, workers := allocateWorkers(ctx, f.channel, f.chaincode, f.count, f.workers, f.client)

	streamId, err := f.client.CreateEventListener(f.channel, f.chaincode)
	if err != nil {
		log.Errorf("Failed to create event listener. %v", err)
		return err
	}

	// prompt the user to start the event listener
	fmt.Printf("Check the fabconnect logs to verify it has subscribed to the events. Press enter to start the transactions...")
	fmt.Scanln()

	err = f.client.StartEventClient(eventAssetIdsChan)
	if err != nil {
		log.Errorf("Failed to start event client. %v", err)
		return err
	}

	// start each worker
	for _, worker := range workers {
		worker.Start()
	}

	assets := make(map[string]bool)
	eventsReceived := 0
	startSet := false
	for eventAssetId := range eventAssetIdsChan {
		log.Infof("Received eventAssetId: %s", eventAssetId)
		assets[eventAssetId] = true
		eventsReceived++
		if eventsReceived >= 2500 && !startSet {
			f.client.Start = time.Now()
			startSet = true
		}
		log.Infof("Events received: %d", eventsReceived)
		if eventsReceived >= f.count {
			break
		}
	}

	printFinalReport(f.count, f.workers, f.client.EventBatchSize, f.client.Start)

	disableCleanup := os.Getenv("NO_CLEANUP")

	if disableCleanup != "true" {
		err = f.client.CleanupEventListener(streamId)
		if err != nil {
			log.Errorf("Failed to cleanup event listener. %v", err)
			return err
		}
	}

	return nil
}
