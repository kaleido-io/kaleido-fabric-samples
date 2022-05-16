package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kaleido-io/kaleido-fabric-go/runners"
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

	initChaincode := os.Getenv("INIT_CC")
	if initChaincode == "" {
		initChaincode = "false"
	} else {
		initChaincode = strings.ToLower(initChaincode)
	}

	var err error
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

	var workers int
	workersStr := os.Getenv("WORKERS")
	if workersStr != "" {
		workers, err = strconv.Atoi(workersStr)
		if err != nil {
			fmt.Printf("Failed to convert %s to integer", workersStr)
			os.Exit(1)
		}
	} else {
		workers = 1
	}
	if workers > 50 {
		fmt.Println("Error: WORKERS cannot exceed 50")
		os.Exit(1)
	}

	init := initChaincode == "true"

	useFabconnect := os.Getenv("USE_FABCONNECT")
	if useFabconnect == "true" {
		runner := runners.NewFabconnectRunner(username, ccname, count, workers, init)
		_ = runner.Exec()
	} else {
		runner := runners.NewSDKRunner(username, ccname, count, workers, init)
		_ = runner.Exec()
	}

	fmt.Printf("\nAll Done!\n")
}
