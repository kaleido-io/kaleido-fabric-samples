package kaleido

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
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

type FabConnectEventStreamPostPayload struct {
	Name      string        `json:"name,omitempty"`
	Type      string        `json:"type,omitempty"`
	BatchSize int           `json:"batchSize,omitempty"`
	WebSocket WebSocketSpec `json:"websocket,omitempty"`
}

type WebSocketSpec struct {
	Topic string `json:"topic,omitempty"`
}

type FabConnectEventStreamPostResponse struct {
	ID string `json:"id,omitempty"`
}

type FabConnectSubscriptionPostPayload struct {
	StreamId    string     `json:"stream,omitempty"`
	Channel     string     `json:"channel,omitempty"`
	Name        string     `json:"name,omitempty"`
	Signer      string     `json:"signer,omitempty"`
	FromBlock   string     `json:"fromBlock,omitempty"`
	PayloadType string     `json:"payloadType,omitempty"`
	Filter      FilterSpec `json:"filter,omitempty"`
}

type FilterSpec struct {
	ChaincodeId string `json:"chaincodeId,omitempty"`
}

type Event struct {
	ChaincodeId string       `json:"chaincodeId"`
	BlockNumber uint64       `json:"blockNumber"`
	TxId        string       `json:"transactionId"`
	TxIndex     int          `json:"transactionIndex"`
	EventIndex  int          `json:"eventIndex"`
	EventName   string       `json:"eventName"`
	Payload     EventPayload `json:"payload"`
}

type EventPayload struct {
	AssetId string `json:"ID"`
}

type ChainInfoResponse struct {
	Result ChainInfo `json:"result"`
}

type ChainInfo struct {
	Height int `json:"height"`
}

type ErrorMessage struct {
	Message string `json:"error"`
}

const (
	EVENT_LISTENER_TOPIC = "fabconnect-perf-topic-1"
)

type FabconnectClient struct {
	r              *resty.Client
	ws             *websocket.Conn
	username       string
	EventBatchSize int
	Start          time.Time
}

func NewFabconnectClient(fabconnectUrl, username string) (*FabconnectClient, error) {
	r := resty.New().SetBaseURL(fabconnectUrl).SetRetryCount(10).AddRetryCondition(func(r *resty.Response, err error) bool {
		if r.StatusCode() > 202 {
			var errMsg ErrorMessage
			err1 := json.Unmarshal(r.Body(), &errMsg)
			if err1 == nil && errMsg.Message == "Too many in-flight transactions" {
				return true
			}
		}
		return false
	})
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:3001/ws", nil)
	if err != nil {
		log.Errorf("Failed to connect to websocket. %v", err)
		return nil, err
	}

	return &FabconnectClient{
		r:        r,
		ws:       conn,
		username: username,
		Start:    time.Now(),
	}, nil
}

func (f *FabconnectClient) EnsureIdentity() error {
	log.Infof("Checking if identity exists: %s", f.username)
	var identity FabconnectIdentity
	getIdentity, err := f.r.R().SetResult(&identity).Get(fmt.Sprintf("/identities/%s", f.username))
	if err != nil {
		log.Errorf("Failed to get identity. %v", err)
		return err
	}

	if getIdentity.StatusCode() == 500 {
		log.Errorf("Error getting identity. %v", getIdentity.String())
		log.Infof("Creating identity: %s", f.username)

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
		log.Infof("Identity already exists: %s", f.username)
	}

	if identity.EnrollmentCert != "" {
		log.Infof("Identity already enrolled: %s", f.username)
		return nil
	}

	if identity.Secret != "" {
		log.Infof("Enrolling identity: %s", f.username)

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

func (f *FabconnectClient) InitChaincode(channel, chaincodeId string) (string, error) {
	receiptId, err := f.sendTransaction(true, channel, chaincodeId, "")
	if err != nil {
		return "", err
	}

	return receiptId, nil
}

func (f *FabconnectClient) ExecChaincode(channel, chaincodeId, assetName string) (string, error) {
	receiptId, err := f.sendTransaction(false, channel, chaincodeId, assetName)
	if err != nil {
		return "", err
	}

	return receiptId, nil
}

func (f *FabconnectClient) sendTransaction(init bool, channel, chaincodeId string, assetName string) (string, error) {
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
			Channel:   channel,
			Chaincode: chaincodeId,
		},
		Func: functionName,
		Args: functionArgs,
		Init: init,
	}
	var transactionConfirmation FabconnectTransactionConfirmation

	sendTx, err := f.r.R().EnableTrace().SetBody(transactionPayload).SetResult(&transactionConfirmation).Post("/transactions?fly-sync=false")
	if err != nil {
		return "", fmt.Errorf("server error message: %s", err.Error())
	}

	if sendTx.StatusCode() != 202 {
		return "", fmt.Errorf("unexpected status code: %s", sendTx.String())
	}

	if !sendTx.Request.TraceInfo().IsConnReused {
		log.Info("== HTTP connection was not reused ==")
	}

	if !transactionConfirmation.Sent {
		return "", fmt.Errorf("transaction not sent successfully. sent = %v", transactionConfirmation.Sent)
	}

	return transactionConfirmation.Id, nil
}

func (f *FabconnectClient) GetReceipt(receiptId string) (*FabconnectTransactionReceipt, error) {
	log.Infof("Getting receipts for %s", receiptId)

	var receipt FabconnectTransactionReceipt
	resp, err := f.r.R().SetResult(&receipt).Get(fmt.Sprintf("/receipts/%s", receiptId))
	if err != nil {
		log.Errorf("Failed to get receipt %s. %v", receiptId, err)
		return nil, err
	} else if resp.StatusCode() != 200 {
		log.Errorf("Status code for getting receipt %s: %d", receiptId, resp.StatusCode())
		return nil, nil
	}

	return &receipt, nil
}

func (f *FabconnectClient) CreateEventListener(channel, chaincodeId string) (string, error) {
	batchStr := os.Getenv("EVENT_BATCH_SIZE")
	if batchStr != "" {
		batchSize, err := strconv.Atoi(batchStr)
		if err != nil {
			log.Errorf("Failed to convert %s to integer", batchStr)
			os.Exit(1)
		}
		f.EventBatchSize = batchSize
	} else {
		f.EventBatchSize = 1
	}

	body := FabConnectEventStreamPostPayload{
		Name:      "fabconnect-perf-1",
		Type:      "websocket",
		BatchSize: f.EventBatchSize,
		WebSocket: WebSocketSpec{
			Topic: EVENT_LISTENER_TOPIC,
		},
	}
	result := FabConnectEventStreamPostResponse{}
	_, err := f.r.R().SetBody(body).SetResult(&result).Post("/eventstreams")
	if err != nil {
		log.Errorf("Failed to create event stream. %v", err)
		return "", err
	}
	streamId := result.ID
	log.Infof("Created event stream: %s", streamId)

	// retrieve block height
	url := fmt.Sprintf("/chainInfo?fly-channel=%s&fly-signer=%s", channel, f.username)
	var chainInfo ChainInfoResponse
	_, err = f.r.R().SetResult(&chainInfo).Get(url)
	if err != nil {
		log.Errorf("Failed to get chain info. %v", err)
		return "", err
	}

	subscriptionBody := FabConnectSubscriptionPostPayload{
		StreamId:    streamId,
		Channel:     channel,
		Name:        "fabconnect-perf-subscription-1",
		Signer:      f.username,
		FromBlock:   strconv.Itoa(chainInfo.Result.Height),
		PayloadType: "json",
		Filter: FilterSpec{
			ChaincodeId: chaincodeId,
		},
	}
	subResult := FabConnectSubscriptionPostPayload{}
	_, err = f.r.R().SetBody(subscriptionBody).SetResult(&subResult).Post("/subscriptions")
	if err != nil {
		log.Errorf("Failed to create subscription. %v", err)
		return "", err
	}

	return streamId, nil
}

func (f *FabconnectClient) CleanupEventListener(eventStreamId string) error {
	log.Infof("Cleaning up event stream: %s", eventStreamId)
	_, err := f.r.R().Delete(fmt.Sprintf("/eventstreams/%s", eventStreamId))
	if err != nil {
		log.Errorf("Failed to delete event stream %s. %v", eventStreamId, err)
		return err
	}

	return nil
}

func (f *FabconnectClient) StartEventClient(assetIdsChan chan string) error {
	done := make(chan struct{})
	err := f.ws.WriteJSON(map[string]string{
		"type":  "listen",
		"topic": EVENT_LISTENER_TOPIC,
	})
	if err != nil {
		log.Errorf("Failed to write to websocket. %v", err)
		return err
	}
	log.Infof("Listening for events")
	go func() {
		defer close(done)
		for {
			_, message, err := f.ws.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}
			events := []Event{}
			err = json.Unmarshal(message, &events)
			if err != nil {
				log.Errorf("Failed to unmarshal event response. %v", err)
				return
			}
			for _, event := range events {
				assetIdsChan <- event.Payload.AssetId
			}
			err = f.ws.WriteJSON(map[string]string{
				"type":  "ack",
				"topic": EVENT_LISTENER_TOPIC,
			})
			if err != nil {
				log.Errorf("Failed to write to websocket. %v", err)
				return
			}
		}
	}()

	return nil
}
