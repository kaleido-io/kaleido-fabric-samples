package kaleido

import (
	"fmt"
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
	r        *resty.Client
	username string
	Start    time.Time
}

func NewFabconnectClient(fabconnectUrl, username string) *FabconnectClient {
	r := resty.New().SetBaseURL(fabconnectUrl)
	return &FabconnectClient{
		r:        r,
		username: username,
		Start:    time.Now(),
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

func (f *FabconnectClient) InitChaincode(chaincodeId string) (string, error) {
	return f.ExecChaincode(true, chaincodeId, "")
}

func (f *FabconnectClient) ExecChaincode(init bool, chaincodeId, assetName string) (string, error) {
	receiptId, err := f.sendTransaction(init, chaincodeId, assetName)
	if err != nil {
		return "", err
	}

	return receiptId, nil
}

func (f *FabconnectClient) sendTransaction(init bool, chaincodeId string, assetName string) (string, error) {
	functionName := "InitLedger"
	functionArgs := []string{}
	if !init {
		functionName = "CreateAsset"
		functionArgs = []string{assetName, "yellow", "10", "Tom", "1300"}
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

func (f *FabconnectClient) GetReceipt(receiptId string) (*FabconnectTransactionReceipt, error) {
	fmt.Printf("Getting receipts for %s\n", receiptId)

	var receipt FabconnectTransactionReceipt
	resp, err := f.r.R().SetResult(&receipt).Get(fmt.Sprintf("/receipts/%s", receiptId))
	if err != nil {
		fmt.Printf("Failed to get receipt %s. %v\n", receiptId, err)
		return nil, err
	} else if resp.StatusCode() != 200 {
		fmt.Printf("Status code for getting receipt %s: %d\n", receiptId, resp.StatusCode())
		return nil, nil
	}

	return &receipt, nil
}
