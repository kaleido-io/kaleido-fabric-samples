package runners

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
)

type FabconnectRunner struct {
	user          string
	chaincode     string
	count         int
	workers       int
	initChaincode bool
	client        *kaleido.FabconnectClient
}

func NewFabconnectRunner(user, chaincode string, count, workers int, initChaincode bool) *FabconnectRunner {
	return &FabconnectRunner{
		user:          user,
		chaincode:     chaincode,
		count:         count,
		workers:       workers,
		initChaincode: initChaincode,
	}
}

func (f *FabconnectRunner) Exec() error {
	fmt.Println("Using Fabconnect for transaction submission")

	fabconnectUrl := os.Getenv("FABCONNECT_URL")
	f.client = kaleido.NewFabconnectClient(fabconnectUrl, f.user)

	err := f.client.EnsureIdentity()
	if err != nil {
		fmt.Printf("Failed to ensure Fabconnect identity. %v\n", err)
		return err
	}
	fmt.Printf("Using Fabconnect identity: %s\n", f.user)

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
	receiptId, err := f.client.InitChaincode(f.chaincode)
	if err != nil {
		fmt.Printf("Failed to initialize chaincode: %s\n", err)
		return err
	}

	receipt, err := f.client.GetReceipt(receiptId)
	if err != nil {
		fmt.Printf("Failed to query receipt %s", receiptId)
		return err
	}

	if receipt.Headers.Type == "TransactionSuccess" {
		fmt.Printf("Chaincode init successful\n")
	}

	return nil
}

func (f *FabconnectRunner) runTransactions() error {
	ctx := context.Background()
	done := make(chan interface{})
	sequence := 0
	for ; sequence < f.workers; sequence++ {
		worker := NewWorker(ctx, f.chaincode, sequence, done, f.client, nil)
		worker.Start(sequence)
	}

	var receiptIds []string
	for {
		receiptId := <-done
		receiptIds = append(receiptIds, receiptId.(string))
		if sequence < f.count {
			worker := NewWorker(ctx, f.chaincode, sequence, done, f.client, nil)
			worker.Start(sequence)
			sequence++
		} else {
			if len(receiptIds) == f.count {
				break
			}
		}
	}

	fmt.Print("\nAll transactions submitted, start tracking receipts\n")

	var receipts []*kaleido.FabconnectTransactionReceipt
	for sequence = 0; sequence < f.workers; sequence++ {
		worker := NewWorker(ctx, f.chaincode, sequence, done, f.client, nil)
		worker.Track(receiptIds[sequence])
	}

	for {
		receipt := <-done
		receipts = append(receipts, receipt.(*kaleido.FabconnectTransactionReceipt))
		if sequence < f.count {
			worker := NewWorker(ctx, f.chaincode, sequence, done, f.client, nil)
			worker.Track(receiptIds[sequence])
			sequence++
		} else {
			if len(receipts) == f.count {
				break
			}
		}
	}

	f.printFinalReport(receipts)

	return nil
}

func (f *FabconnectRunner) printFinalReport(receipts []*kaleido.FabconnectTransactionReceipt) {
	fmt.Println("\n\nFinal Report")
	totalSuccesses := 0
	totalFailures := 0
	maxTotalTime := 0.0
	maxTime := 0.0
	for i := 0; i < len(receipts); i++ {
		tr := receipts[i]
		if tr.Headers.Type == "TransactionSuccess" {
			totalSuccesses++
		} else {
			totalFailures++
		}
		te := tr.Headers.TimeElapsed
		if te > maxTime {
			maxTime = te
			if te > maxTotalTime {
				maxTotalTime = te
			}
		}
	}
	fmt.Printf("  - Total program runtime: %s\n", time.Since(f.client.Start))
	fmt.Printf("  - Total transaction successes: %d\n", totalSuccesses)
	fmt.Printf("  - Total transaction failures: %d\n", totalFailures)
	fmt.Printf("  - Longest transaction TimeElapsed: %f\n", maxTotalTime)
}
