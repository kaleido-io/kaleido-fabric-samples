package runners

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
	log "github.com/sirupsen/logrus"
)

type FabricClient interface {
	InitChaincode(channel, chaincodeId string) (string, error)
	ExecChaincode(channel, chaincodeId, assetId string) (string, error)
}

type Worker interface {
	SetClient(FabricClient)
	IncreaseTxCount()
	Start()
	// Track(string)
}

type worker struct {
	channel      string
	chaincode    string
	index        int
	txCount      int
	ctx          context.Context
	eventAssetId chan kaleido.TxResult
	client       FabricClient
}

func NewWorker(ctx context.Context, channel, ccname string, index int, eventAssetId chan kaleido.TxResult) Worker {
	w := &worker{
		channel:      channel,
		chaincode:    ccname,
		index:        index,
		eventAssetId: eventAssetId,
		ctx:          ctx,
	}
	return w
}

var TIMEOUT time.Duration = time.Duration(60) * time.Second

func (w *worker) SetClient(client FabricClient) {
	w.client = client
}

func (w *worker) Start() {
	go func() {
		// for each tx count, send a transaction
		for i := 0; i < w.txCount; i++ {
			newId, err := generateId()
			if err != nil {
				log.Errorf("[worker:%d] Failed to generate asset ID. %s", w.index, err)
				panic(err)
			}
			assetId := fmt.Sprintf("asset-%s", newId)
			log.Infof("[worker:%d] Send transaction %d of %d (%s)", w.index, i+1, w.txCount, assetId)
			id, err := w.client.ExecChaincode(w.channel, w.chaincode, assetId)
			if err != nil {
				log.Errorf("[worker:%d] Failed to send transaction %d of %d (%s). %s", w.index, i+1, w.txCount, assetId, err)
				w.eventAssetId <- kaleido.TxResult{AssetId: assetId, Success: false}
			} else {
				log.Infof("[worker:%d] Transaction %d of %d (%s) sent. Asset ID: %s", w.index, i+1, w.txCount, assetId, id)
				w.eventAssetId <- kaleido.TxResult{AssetId: assetId, Success: true}
			}
		}
	}()
}

func (w *worker) IncreaseTxCount() {
	w.txCount++
}

// func (w *worker) Track(receiptId string) {
// 	ticker := time.NewTicker(1 * time.Second)

// 	go func(id string) {
// 		ctx, cancel := context.WithTimeout(w.ctx, TIMEOUT)
// 		defer cancel()

// 		complete := false
// 		for {
// 			if complete {
// 				break
// 			}
// 			select {
// 			case <-ticker.C:
// 				receipt, err := w.fabconnectClient.GetReceipt(id)
// 				if err == nil && receipt != nil {
// 					complete = true
// 					w.done <- receipt
// 				}
// 			case <-ctx.Done():
// 				complete = true
// 				w.done <- nil
// 			}
// 		}
// 	}(receiptId)
// }

func generateId() (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base32.HexEncoding.EncodeToString(randomBytes)[:10], nil
}

func allocateWorkers(ctx context.Context, channel, chaincode string, txCount, numWorkers int, client FabricClient) (chan kaleido.TxResult, []Worker) {
	eventAssetIdsChan := make(chan kaleido.TxResult)
	sequence := 0
	workers := make([]Worker, numWorkers)
	for ; sequence < numWorkers; sequence++ {
		worker := NewWorker(ctx, channel, chaincode, sequence, eventAssetIdsChan)
		worker.SetClient(client)
		workers[sequence] = worker
	}

	for index := 0; index < txCount; index++ {
		workerIdx := index % numWorkers
		w := workers[workerIdx]
		w.IncreaseTxCount()
	}
	return eventAssetIdsChan, workers
}

func printFinalReport(txCount, numWorkers, eventBatchSize int, startTime time.Time) {
	fmt.Println("\n\nFinal Report")
	fmt.Println("  - Configuration:")
	fmt.Printf("    * total transactions: %d\n", txCount)
	fmt.Printf("    * workers count: %d\n", numWorkers)
	fmt.Printf("    * event batch size: %d\n", eventBatchSize)
	elapsed := time.Since(startTime)
	fmt.Printf("  - Total program runtime: %s\n", elapsed)
	fmt.Printf("  - TPS: %f\n", float64(txCount)/elapsed.Seconds())
}
