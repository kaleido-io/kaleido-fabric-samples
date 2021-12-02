package kaleido

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	kld "github.com/kaleido-io/kaleido-sdk-go/kaleido"
)

type KaleidoNetwork struct {
	Consortium   kld.Consortium
	Environment  kld.Environment
	Memberships  []kld.Membership
	MyMembership kld.Membership
	MyCA         kld.Service
	Orderers     []kld.Node
	Peers        []kld.Node

	client kld.KaleidoClient
}

func NewNetwork() *KaleidoNetwork {
	kn := KaleidoNetwork{}
	return &kn
}

func (kn *KaleidoNetwork) Initialize() {
	fmt.Println("Initializing Kaleido Network...")

	apikey := os.Getenv("APIKEY")
	if apikey == "" {
		fmt.Println("Must set environment variable \"APIKEY\"")
		os.Exit(1)
	}

	url := getApiUrl()
	fmt.Printf("URL: %v\n", url)

	kn.client = kld.NewClient(url, apikey)

	kn.selectConsortium()
	kn.selectEnvironment()
	kn.selectMembership()
	kn.getMyCA()
	kn.getOrderersAndPeers()
}

func (kn *KaleidoNetwork) selectConsortium() {
	var consortiums []kld.Consortium
	var targetCon kld.Consortium

	_, err := kn.client.ListConsortium(&consortiums)
	if err != nil {
		fmt.Printf("Failed to get list of consortiums. %v\n", err)
		os.Exit(1)
	}
	liveConsortiums := []kld.Consortium{}
	for _, con := range consortiums {
		if con.State != "deleted" {
			liveConsortiums = append(liveConsortiums, con)
		}
	}

	requestedConsortium := os.Getenv("CONSORTIUM")
	if len(liveConsortiums) == 0 {
		fmt.Printf("No consortium found using URL\n")
		os.Exit(1)
	} else if requestedConsortium != "" {
		for _, consortium := range consortiums {
			if consortium.ID == requestedConsortium {
				targetCon = consortium
			}
		}
		if targetCon.ID != requestedConsortium {
			fmt.Printf("No consortium found matching id=%s\n", requestedConsortium)
			os.Exit(1)
		}
	} else if len(liveConsortiums) > 1 {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Select the target consortium:")
		for i, con := range liveConsortiums {
			fmt.Printf("\t[%v] %v (%v,id=%s)\n", i, con.Name, con.State, con.ID)
		}

		for {
			fmt.Print("-> ")
			text, _ := reader.ReadString('\n')
			// convert CRLF to LF
			text = strings.Replace(text, "\n", "", -1)

			i, _ := strconv.Atoi(text)
			targetCon = liveConsortiums[i]
			break
		}
	} else {
		targetCon = liveConsortiums[0]
	}

	fmt.Printf("Target consortium: %v (id=%v)\n", targetCon.Name, targetCon.ID)
	kn.Consortium = targetCon
}

func (kn *KaleidoNetwork) selectEnvironment() {
	var envs []kld.Environment
	var targetEnv kld.Environment

	_, err := kn.client.ListEnvironments(kn.Consortium.ID, &envs)
	if err != nil {
		fmt.Printf("Failed to get list of environments. %v\n", err)
		os.Exit(1)
	}
	liveEnvs := []kld.Environment{}
	for _, env := range envs {
		if env.State != "deleted" && env.Provider == "fabric" {
			liveEnvs = append(liveEnvs, env)
		}
	}

	requestedEnvironment := os.Getenv("ENVIRONMENT")
	if len(liveEnvs) == 0 {
		fmt.Printf("No Fabric environments found in consortium %s\n", kn.Consortium.ID)
		os.Exit(1)
	} else if requestedEnvironment != "" {
		for _, env := range liveEnvs {
			if env.ID == requestedEnvironment {
				targetEnv = env
			}
		}
		if targetEnv.ID != requestedEnvironment {
			fmt.Printf("No environment found matching id=%s in consortium %s\n", requestedEnvironment, kn.Consortium.ID)
			os.Exit(1)
		}
	} else if len(liveEnvs) > 1 {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Select the target environment:")
		for i, env := range liveEnvs {
			fmt.Printf("\t[%v] %v (%v)\n", i, env.Name, env.State)
		}

		for {
			fmt.Print("-> ")
			text, _ := reader.ReadString('\n')
			// convert CRLF to LF
			text = strings.Replace(text, "\n", "", -1)

			i, _ := strconv.Atoi(text)
			targetEnv = liveEnvs[i]
			break
		}
	} else {
		targetEnv = liveEnvs[0]
	}

	fmt.Printf("Target environment: %v (id=%v)\n", targetEnv.Name, targetEnv.ID)
	kn.Environment = targetEnv
}

func (kn *KaleidoNetwork) selectMembership() {
	var memberships []kld.Membership
	var targetMembership kld.Membership

	_, err := kn.client.ListMemberships(kn.Consortium.ID, &memberships)
	if err != nil {
		fmt.Printf("Failed to get list of memberships. %v\n", err)
		os.Exit(1)
	}

	desiredMembership := os.Getenv("SUBMITTER")
	if len(memberships) > 1 && desiredMembership == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Select the membership to submit transactions from:")
		for i, mem := range memberships {
			fmt.Printf("\t[%v] %v (%v)\n", i, mem.OrgName, mem.ID)
		}

		for {
			fmt.Print("-> ")
			text, _ := reader.ReadString('\n')
			// convert CRLF to LF
			text = strings.Replace(text, "\n", "", -1)

			i, _ := strconv.Atoi(text)
			targetMembership = memberships[i]
			break
		}
	} else {
		if desiredMembership != "" {
			for _, mem := range memberships {
				if mem.ID == desiredMembership {
					targetMembership = mem
					break
				}
			}
		} else {
			targetMembership = memberships[0]
		}
	}

	fmt.Printf("Target membership: %v (id=%v)\n", targetMembership.OrgName, targetMembership.ID)
	kn.MyMembership = targetMembership
	kn.Memberships = memberships
}

func (kn *KaleidoNetwork) getOrderersAndPeers() {
	var nodes []kld.Node

	peers := []kld.Node{}
	orderers := []kld.Node{}

	_, err := kn.client.ListNodes(kn.Consortium.ID, kn.Environment.ID, &nodes)
	if err != nil {
		fmt.Printf("Failed to get list of nodes. %v\n", err)
		os.Exit(1)
	}
	if len(nodes) == 0 {
		fmt.Println("The environment does not have any orderers or peers.")
		os.Exit(1)
	}

	for i := range nodes {
		if nodes[i].Role == "orderer" {
			fmt.Printf("Found orderer %s (membership=%s)\n", nodes[i].ID, nodes[i].MembershipID)
			orderers = append(orderers, nodes[i])
		} else if nodes[i].Role == "peer" {
			fmt.Printf("Found peer %s (membership=%s)\n", nodes[i].ID, nodes[i].MembershipID)
			peers = append(peers, nodes[i])
		}
	}

	if len(orderers) == 0 {
		fmt.Println("The environment does not have any orderers")
		os.Exit(1)
	}
	if len(peers) == 0 {
		fmt.Println("The environment does not have any peers")
		os.Exit(1)
	}

	kn.Orderers = orderers
	kn.Peers = peers
}

func (kn *KaleidoNetwork) getMyCA() {
	var services []kld.Service

	_, err := kn.client.ListServices(kn.Consortium.ID, kn.Environment.ID, &services)
	if err != nil {
		fmt.Printf("Failed to get list of services. %v\n", err)
		os.Exit(1)
	}
	if len(services) == 0 {
		fmt.Println("The environment does not have any services.")
		os.Exit(1)
	}

	for i := range services {
		if services[i].Service == "fabric-ca" && services[i].MembershipID == kn.MyMembership.ID {
			fmt.Printf("Found fabric-ca service %s for my membership %s\n", services[i].ID, kn.MyMembership.ID)
			kn.MyCA = services[i]
			return
		}
	}
	fmt.Printf("Failed to locate fabric-ca service for my membership %s\n", kn.MyMembership.ID)
	os.Exit(1)
}

func getApiUrl() string {
	url := os.Getenv("KALEIDO_URL")
	if url == "" {
		url = "https://console.kaleido.io/api/v1"
	}
	return url
}
