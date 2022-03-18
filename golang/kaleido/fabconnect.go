package kaleido

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

type FabconnectIdentity struct {
	Name           string `json:"name,omitempty"`
	EnrollmentCert string `json:"enrollmentCert,omitempty"`
	Secret         string `json:"secret,omitempty"`
}

type FabconnectIdentityPayload struct {
	Name           string                 `json:"name,omitempty"`
	Type           string                 `json:"type,omitempty"`
	MaxEnrollments int                    `json:"maxEnrollments,omitempty"`
	Attributes     map[string]interface{} `json:"attributes,omitempty"`
}

type FabconnectEnrollIdentityPayload struct {
	Secret     string                 `json:"secret,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type FabconnectTransactionPayloadHeaders struct {
	Type      string `json:"type,omitempty"`
	Signer    string `json:"signer,omitempty"`
	Channel   string `json:"channel,omitempty"`
	Chaincode string `json:"chaincode,omitempty"`
}

type FabconnectTransactionPayload struct {
	Headers FabconnectTransactionPayloadHeaders `json:"headers,omitempty"`
	Func    string                              `json:"func,omitempty"`
	Args    []string                            `json:"args,omitempty"`
	Init    bool                                `json:"init,omitempty"`
}

type FabconnectTransactionConfirmation struct {
	Sent bool   `json:"sent,omitempty"`
	Id   string `json:"id,omitempty"`
}

type FabconnectTransactionReceiptHeaders struct {
	TimeElapsed float64 `json:"timeElapsed,omitempty"`
	Type        string  `json:"type,omitempty"`
}

type FabconnectTransactionReceipt struct {
	Id      string                              `json:"_id,omitempty"`
	Headers FabconnectTransactionReceiptHeaders `json:"headers,omitempty"`
	Status  string                              `json:"status,omitempty"`
}

type FabconnectClient struct {
	r              *resty.Client
	username       string
	batchTxMap     map[int]map[string]FabconnectTransactionReceipt
	batchTxMapLock sync.Mutex
	start          time.Time
}

func NewFabconnectClient(fabconnectUrl, username string) *FabconnectClient {
	r := resty.New().SetBaseURL(fabconnectUrl)
	return &FabconnectClient{
		r:              r,
		username:       username,
		batchTxMap:     make(map[int]map[string]FabconnectTransactionReceipt),
		batchTxMapLock: sync.Mutex{},
		start:          time.Now(),
	}
}

func (f *FabconnectClient) EnsureIdentity() error {
	fmt.Printf("Checking if identity exists: %s\n", f.username)
	var identity FabconnectIdentity
	getIdentity, err := f.r.R().SetResult(&identity).Get(fmt.Sprintf("/identities/%s", f.username))
	if err != nil {
		fmt.Printf("Failed to get identity. %v\n", err)
		return err
	}

	if getIdentity.StatusCode() == 500 {
		fmt.Printf("Error getting identity. %v\n", getIdentity.String())
		fmt.Printf("Creating identity: %s\n", f.username)

		identityPayload := FabconnectIdentityPayload{
			Name:           f.username,
			Type:           "client",
			MaxEnrollments: 0,
			Attributes:     make(map[string]interface{}),
		}

		_, err := f.r.R().SetBody(identityPayload).SetResult(&identity).Post("/identities")
		if err != nil {
			return fmt.Errorf("failed to create identity. %v", err)
		}
	} else {
		fmt.Printf("Identity already exists: %s\n", f.username)
	}

	if identity.EnrollmentCert != "" {
		fmt.Printf("Identity already enrolled: %s\n", f.username)
		return nil
	}

	if identity.Secret != "" {
		fmt.Printf("Enrolling identity: %s\n", f.username)

		enrollIdentityPayload := FabconnectEnrollIdentityPayload{
			Secret:     identity.Secret,
			Attributes: make(map[string]interface{}),
		}
		_, err := f.r.R().SetBody(enrollIdentityPayload).Post(fmt.Sprintf("/identities/%s/enroll", f.username))
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("missing enrollment secret for identity: %s", f.username)
	}

	return nil
}

func (f *FabconnectClient) InitChaincode(chaincodeId string) error {
	return f.ExecChaincode(true, 0, chaincodeId, "")
}

func (f *FabconnectClient) ExecChaincode(init bool, batch int, chaincodeId, assetName string) error {
	transactionId, err := f.sendTransaction(init, chaincodeId, assetName)
	if err != nil {
		return err
	}

	f.batchTxMapLock.Lock()
	if f.batchTxMap[batch] == nil {
		f.batchTxMap[batch] = make(map[string]FabconnectTransactionReceipt)
	}
	f.batchTxMap[batch][transactionId] = FabconnectTransactionReceipt{}
	f.batchTxMapLock.Unlock()

	return nil
}

func (f *FabconnectClient) sendTransaction(init bool, chaincodeId string, assetName ...string) (string, error) {
	functionName := "InitLedger"
	functionArgs := []string{}
	if !init {
		functionName = "CreateAsset"
		functionArgs = []string{assetName[0], "yellow", "10", "Tom", "1300"}
	}

	transactionPayload := FabconnectTransactionPayload{
		Headers: FabconnectTransactionPayloadHeaders{
			Type:      "SendTransaction",
			Signer:    f.username,
			Channel:   "default-channel",
			Chaincode: chaincodeId,
		},
		Func: functionName,
		Args: functionArgs,
		Init: init,
	}
	var transactionConfirmation FabconnectTransactionConfirmation

	sendTx, err := f.r.R().EnableTrace().SetBody(transactionPayload).SetResult(&transactionConfirmation).Post("/transactions?fly-sync=false")
	if err != nil {
		return "", fmt.Errorf(err.Error())
	}

	if sendTx.StatusCode() != 202 {
		return "", fmt.Errorf(sendTx.String())
	}

	if !sendTx.Request.TraceInfo().IsConnReused {
		fmt.Println("== HTTP connection was not reused ==")
	}

	if !transactionConfirmation.Sent {
		return "", fmt.Errorf("transaction not sent successfully. sent = %v", transactionConfirmation.Sent)
	}

	return transactionConfirmation.Id, nil
}

func (f *FabconnectClient) MonitorBatch(batch int) error {
	fmt.Printf("Batch %d contains %d successfully submitted transactions\n", batch+1, len(f.batchTxMap[batch]))

	for {
		fmt.Printf("Waiting 5 seconds for polling for receipts from batch %d\n", batch+1)
		time.Sleep(5 * time.Second)

		err := f.getReceipts(batch, f.batchTxMap[batch])
		if err != nil {
			return err
		}

		successfulReceipts := make([]FabconnectTransactionReceipt, 0)

		continuePolling := false
		for k, ftr := range f.batchTxMap[batch] {
			if ftr.Headers.Type == "TransactionSuccess" {
				successfulReceipts = append(successfulReceipts, ftr)
			} else {
				continuePolling = continuePolling || ftr.Headers.Type != "Error"
				fmt.Printf("  - Batch %d transaction %s ERROR. TimeElapsed: %f, Type: %s\n", batch+1, k, ftr.Headers.TimeElapsed, ftr.Headers.Type)
			}
		}

		sort.Slice(successfulReceipts, func(i, j int) bool {
			return successfulReceipts[i].Headers.TimeElapsed < successfulReceipts[j].Headers.TimeElapsed
		})
		for _, ftr := range successfulReceipts {
			fmt.Printf("  - Batch %d transaction %s SUCCESS. TimeElapsed: %f\n", batch+1, ftr.Id, ftr.Headers.TimeElapsed)
		}

		if !continuePolling {
			fmt.Printf("All transaction receipts received from batch %d\n", batch+1)
			break
		}
	}

	return nil
}

func (f *FabconnectClient) PrintFinalReport(batches int) {
	fmt.Println("\n\nFinal Report")
	totalFailures := 0
	maxTotalTime := 0.0
	maxFromBatch := 0
	for i := 0; i < batches; i++ {
		successfulReceipts := 0
		maxTime := 0.0
		for _, ftr := range f.batchTxMap[i] {
			if ftr.Headers.Type == "TransactionSuccess" {
				successfulReceipts++
			} else {
				totalFailures++
			}
			te := ftr.Headers.TimeElapsed
			if te > maxTime {
				maxTime = te
				if te > maxTotalTime {
					maxTotalTime = te
					maxFromBatch = i + 1
				}
			}
		}
		fmt.Printf("  - Batch %d: %d of %d submitted transactions were successful. Longest TimeElapsed: %f\n", i+1, successfulReceipts, len(f.batchTxMap[i]), maxTime)
	}
	fmt.Printf("  - Total program runtime: %s\n", time.Since(f.start))
	fmt.Printf("  - Total batches: %d\n", batches)
	fmt.Printf("  - Transactions per batch: %s\n", os.Getenv("TX_COUNT"))
	fmt.Printf("  - Total transaction failures from all batches: %d\n", totalFailures)
	fmt.Printf("  - Longest transaction TimeElapsed from all batches: %f from batch %d\n", maxTotalTime, maxFromBatch)
}

func (f *FabconnectClient) getReceipts(batch int, batchTransactions map[string]FabconnectTransactionReceipt) error {
	ids := make([]string, 0, len(batchTransactions))
	for k, ftr := range batchTransactions {
		if ftr.Headers.Type != "TransactionSuccess" {
			ids = append(ids, k)
		}
	}

	fmt.Printf("Getting receipts for %d unverified transactions in batch %d\n", len(ids), batch+1)

	var receipts []FabconnectTransactionReceipt
	_, err := f.r.R().SetResult(&receipts).Get(fmt.Sprintf("/receipts?id=%s", strings.Join(ids, "&id=")))
	if err != nil {
		fmt.Printf("Failed to get receipts. %v\n", err)
		return err
	}

	f.batchTxMapLock.Lock()
	for _, ftr := range receipts {
		batchTransactions[ftr.Id] = FabconnectTransactionReceipt{
			Id:      ftr.Id,
			Headers: ftr.Headers,
			Status:  ftr.Status,
		}
	}
	f.batchTxMapLock.Unlock()

	return nil
}
