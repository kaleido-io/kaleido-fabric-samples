package fabric

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/kaleido-io/kaleido-fabric-go/kaleido"
	kld "github.com/kaleido-io/kaleido-sdk-go/kaleido"
	"gopkg.in/yaml.v2"
)

type FabricNodeInfo struct {
	OrgCA   string `json:"orgCA"`
	TlsCert string `json:"tlsCert"`
}

func BuildConfig(kn *kaleido.KaleidoNetwork) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	m["version"] = "1.1.0"
	client := make(map[string]interface{})

	client["organization"] = kn.MyMembership.ID

	logging := make(map[string]string)
	logging["level"] = "info"
	client["logging"] = logging

	cryptoconf := make(map[string]string)
	homedir, _ := os.UserHomeDir()
	mspdir := path.Join(homedir, "kaleido-fabric-go", kn.Environment.ID, "msp")
	_, err := os.Stat(mspdir)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(mspdir, 0755)
		if errDir != nil {
			return nil, fmt.Errorf("Failed to create msp dir %s", mspdir)
		}
	}
	cryptoconf["path"] = mspdir
	client["cryptoconfig"] = cryptoconf

	bccsp := make(map[string]interface{})
	security := make(map[string]interface{})
	security["enabled"] = true
	defaultSec := make(map[string]string)
	defaultSec["provider"] = "SW"
	security["default"] = defaultSec
	security["hashAlgorithm"] = "SHA2"
	security["softVerify"] = true
	security["level"] = 256
	bccsp["security"] = security
	client["BCCSP"] = bccsp

	cryptostore := make(map[string]string)
	cryptostore["path"] = mspdir
	credstore := make(map[string]interface{})
	credstore["path"] = mspdir
	credstore["cryptoStore"] = cryptostore
	client["credentialStore"] = credstore

	m["client"] = client

	orgs := make(map[string]interface{})
	for i := range kn.Memberships {
		membership := kn.Memberships[i]
		mem := make(map[string]interface{})
		mem["mspid"] = membership.ID
		peers := []string{}
		for j := range kn.Peers {
			if kn.Peers[j].MembershipID == membership.ID {
				if kn.Peers[j].Urls["peer"] != nil {
					hostname := strings.TrimPrefix(kn.Peers[j].Urls["peer"].(string), "https://")
					peers = append(peers, hostname)
				}
			}
		}
		mem["peers"] = peers
		mem["cryptoPath"] = "/tmp/msp"
		if membership.ID == kn.MyMembership.ID {
			mem["certificateAuthorities"] = []string{kn.MyCA.MembershipID}
		}
		orgs[membership.ID] = mem
	}
	m["organizations"] = orgs

	channels := make(map[string]interface{})
	defaultChannel := make(map[string]interface{})

	channelPeers := make(map[string]interface{})
	peers := make(map[string]interface{})
	for i := range kn.Peers {
		if kn.Peers[i].Urls["peer"] != nil {
			hostname := strings.TrimPrefix(kn.Peers[i].Urls["peer"].(string), "https://")
			cpeer := make(map[string]bool)
			cpeer["endorsingPeer"] = true
			cpeer["chaincodeQuery"] = true
			cpeer["ledgerQuery"] = true
			cpeer["eventSource"] = true
			channelPeers[hostname] = cpeer

			opeer := make(map[string]interface{})
			opeer["url"] = hostname + ":443"

			caPemFile, err := saveNodeTlsCA(kn.Peers[i], kn.Environment)
			if err != nil {
				return nil, err
			}

			tlsCACerts := make(map[string]string)
			tlsCACerts["path"] = caPemFile
			opeer["tlsCACerts"] = tlsCACerts
			peers[hostname] = opeer
		}
	}
	defaultChannel["peers"] = channelPeers
	channels["default-channel"] = defaultChannel
	m["channels"] = channels
	m["peers"] = peers

	channelOrderers := make([]string, 1)
	orderers := make(map[string]interface{})
	for i := range kn.Orderers {
		if kn.Orderers[i].MembershipID == kn.MyMembership.ID {
			if kn.Orderers[i].Urls["orderer"] != nil {
				hostname := strings.TrimPrefix(kn.Orderers[i].Urls["orderer"].(string), "https://")
				orderer := make(map[string]interface{})
				orderer["url"] = hostname + ":443"

				caPemFile, err := saveNodeTlsCA(kn.Orderers[i], kn.Environment)
				if err != nil {
					return nil, err
				}

				tlsCACerts := make(map[string]string)
				tlsCACerts["path"] = caPemFile
				orderer["tlsCACerts"] = tlsCACerts
				orderers[hostname] = orderer
				channelOrderers[0] = hostname
				break
			}
		}
	}
	defaultChannel["orderers"] = channelOrderers
	m["orderers"] = orderers

	certAuthorities := make(map[string]interface{})
	myCA := make(map[string]interface{})
	myCA["url"] = kn.MyCA.Urls["http"].(string)
	tlsCACerts := make(map[string]string)
	currentdir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	tlsCACerts["path"] = path.Join(currentdir, "resources/kaleido_ca.pem")
	myCA["tlsCACerts"] = tlsCACerts
	certAuthorities[kn.MyCA.MembershipID] = myCA
	m["certificateAuthorities"] = certAuthorities

	return m, nil
}

func AddTlsConfig(configmap map[string]interface{}, signer msp.SigningIdentity) {
	filename := fmt.Sprintf("%s@%s-cert.pem", signer.Identifier().ID, signer.Identifier().MSPID)
	client := configmap["client"].(map[string]interface{})
	cryptoconfig := client["cryptoconfig"].(map[string]string)
	cryptoconfigpath := cryptoconfig["path"]
	certpath := path.Join(cryptoconfigpath, filename)

	filename = fmt.Sprintf("%s_sk", hex.EncodeToString(signer.PrivateKey().SKI()))
	keypath := path.Join(cryptoconfigpath, "keystore", filename)

	tlsCerts := make(map[string]interface{})
	clientAuth := make(map[string]interface{})
	key := make(map[string]string)
	key["path"] = keypath
	clientAuth["key"] = key
	cert := make(map[string]string)
	cert["path"] = certpath
	clientAuth["cert"] = cert
	tlsCerts["client"] = clientAuth
	client["tlsCerts"] = tlsCerts
}

func NewConfigProvider(configmap map[string]interface{}) (core.ConfigProvider, error) {
	raw, err := yaml.Marshal(configmap)
	if err != nil {
		fmt.Printf("Failed to encode network configuration map into YAML bytes: %v\n", err)
		return nil, err
	}

	fmt.Printf("\n%s\n", string(raw))
	return config.FromRaw(raw, "yaml"), nil
}

func saveNodeTlsCA(node kld.Node, env kld.Environment) (string, error) {
	decoded, _ := hex.DecodeString(node.NodeIdentity)
	var nodeIdentity FabricNodeInfo
	err := json.Unmarshal(decoded, &nodeIdentity)
	if err != nil {
		return "", fmt.Errorf("Failed to parse node info for peer %s. %s", node.ID, err)
	}

	homedir, _ := os.UserHomeDir()
	nodeMspPath := path.Join(homedir, "kaleido-fabric-go", env.ID, "nodeMSPs", node.ID)
	_, err = os.Stat(nodeMspPath)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(nodeMspPath, 0755)
		if errDir != nil {
			return "", fmt.Errorf("Failed to create node msp dir %s", nodeMspPath)
		}
	}
	caPemFile := path.Join(nodeMspPath, "ca.pem")
	err = ioutil.WriteFile(caPemFile, []byte(nodeIdentity.OrgCA), 0644)
	if err != nil {
		return "", fmt.Errorf("Failed to write tls CA PEM for node %s. %s", node.ID, err)
	}

	return caPemFile, nil
}
