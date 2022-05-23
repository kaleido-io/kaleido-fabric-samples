package runners

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
)

type Worker interface {
	Start(int)
	Track(string)
}

type worker struct {
	chaincode string
	index     int
	ctx       context.Context
	done      chan interface{}
	client    *kaleido.FabconnectClient
	channel   *kaleido.Channel
}

func NewWorker(ctx context.Context, ccname string, index int, done chan interface{}, client *kaleido.FabconnectClient, channel *kaleido.Channel) Worker {
	w := &worker{
		client:    client,
		channel:   channel,
		chaincode: ccname,
		index:     index,
		done:      done,
		ctx:       ctx,
	}
	return w
}

var TIMEOUT time.Duration = time.Duration(60) * time.Second

func (w *worker) Start(sequence int) {
	go func() {
		newId, err := generateId()
		if err != nil {
			w.done <- ""
			return
		}
		assetId := fmt.Sprintf("asset-%s", newId)
		fmt.Printf("[worker:%d] Send transaction %d (%s)\n", w.index, sequence, assetId)
		var id string
		label := "Receipt"
		if w.client != nil {
			// receipt id
			id, err = w.client.ExecChaincode(false, w.chaincode, assetId)
		} else {
			// transaction id
			label = "Transaction"
			id, err = w.channel.ExecChaincode(w.chaincode, assetId)
		}
		if err != nil {
			fmt.Printf("[worker:%d] Failed to send transaction %d (%s). %s\n", w.index, sequence, assetId, err)
		} else {
			fmt.Printf("[worker:%d] Transaction %d (%s) sent. %s ID: %s\n", w.index, sequence, assetId, label, id)
		}

		w.done <- id
	}()
}

func (w *worker) Track(receiptId string) {
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		ctx, cancel := context.WithTimeout(w.ctx, TIMEOUT)
		defer cancel()

		complete := false
		for {
			if complete {
				break
			}
			select {
			case <-ticker.C:
				receipt, err := w.client.GetReceipt(receiptId)
				if err == nil && receipt != nil {
					complete = true
					w.done <- receipt
				}
			case <-ctx.Done():
				complete = true
				w.done <- nil
			}
		}
	}()
}

func generateId() (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base32.HexEncoding.EncodeToString(randomBytes)[:10], nil
}
