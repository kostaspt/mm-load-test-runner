package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	runHash string // A random hash for this run

	serverDir   string // Path to the server repository
	loadTestDir string // Path to the mattermost-load-test-ng repository

	serverBaseBranch string // Branch to checkout before building the server for base.out
	serverNewBranch  string // Branch to checkout before building the server for new.out

	loadTestBaseBranch string // Branch to checkout before building the loadtest for base.out
	loadTestNewBranch  string // Branch to checkout before building the loadtest for new.out

	clusterName    string        // Name of the cluster used for the load test
	durationTotal  time.Duration // Total duration of the load test
	durationWindow time.Duration // Duration to use of the load test (now-durationWindow)
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	randomBytes := make([]byte, 4)
	if _, err = rand.Read(randomBytes); err != nil {
		log.Fatal("Error generating random hash")
	}
	runHash = hex.EncodeToString(randomBytes)

	serverDir = os.Getenv("SERVER_DIR")
	loadTestDir = os.Getenv("LOAD_TEST_DIR")
	serverBaseBranch = os.Getenv("SERVER_BASE_BRANCH")
	serverNewBranch = os.Getenv("SERVER_NEW_BRANCH")
	loadTestBaseBranch = os.Getenv("LOAD_TEST_BASE_BRANCH")
	loadTestNewBranch = os.Getenv("LOAD_TEST_NEW_BRANCH")
	clusterName = os.Getenv("CLUSTER_NAME")
	durationTotal, err = time.ParseDuration(os.Getenv("DURATION_TOTAL"))
	if err != nil {
		log.Fatal(err)
	}
	durationWindow, err = time.ParseDuration(os.Getenv("DURATION_OFFSET"))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}

	log.Println("Done!")
}

func run() error {
	ctx := context.Background()

	var actions = []func(context.Context) error{
		// Make sure the server is built from scratch
		destroyDeployment,

		// Checkout the server base branch, build it, and ship it
		checkoutServerBranch(serverBaseBranch),
		buildServer,
		checkoutLoadTestBranch(loadTestBaseBranch),
		buildLoadTest,
		createDeployment,
		initSSHAccess,

		// Run the load test for base
		//stopLoadTest,
		//resetLoadTest,
		setupDatabase,
		forceRestartService, // to make sure the server is reset after initial data
		runLoadTest("base"),

		// Make sure the new test fresh
		destroyDeployment,

		// Checkout the server new branch, build it, and ship it
		checkoutServerBranch(serverNewBranch),
		buildServer,
		checkoutLoadTestBranch(loadTestNewBranch),
		buildLoadTest,
		createDeployment,
		initSSHAccess,

		// Run the load test for new
		//stopLoadTest,
		//resetLoadTest,
		setupDatabase,
		forceRestartService, // to make sure the server is reset after initial data
		runLoadTest("new"),

		// Compare the results at the end
		compareResults,

		// Cleanup
		destroyDeployment,
	}

	for i, action := range actions {
		if err := action(ctx); err != nil {
			return fmt.Errorf("error running action %d: %s", i, err)
		}
	}

	return nil
}
