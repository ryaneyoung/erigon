package node

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/urfave/cli"

	"github.com/ledgerwatch/erigon-lib/common/dbg"
	"github.com/ledgerwatch/erigon/cmd/devnet/models"
	"github.com/ledgerwatch/erigon/cmd/devnet/requests"
	"github.com/ledgerwatch/erigon/cmd/devnet/utils"
	"github.com/ledgerwatch/erigon/params"
	erigonapp "github.com/ledgerwatch/erigon/turbo/app"
	erigoncli "github.com/ledgerwatch/erigon/turbo/cli"
	"github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/log/v3"
)

// Holds the number id of each node on the network, the first node is node 0
var nodeNumber int

// Start starts the process for two erigon nodes running on the dev chain
func Start(wg *sync.WaitGroup) {
	// add one goroutine to the wait-list
	wg.Add(1)

	// start the first node
	go StartNode(wg, miningNodeArgs(), &nodeNumber)

	// sleep for a while to allow first node to start
	time.Sleep(time.Second * 10)

	// get the enode of the first node
	enode, err := getEnode()
	if err != nil {
		// TODO: Log the error, it means node did not start well
		fmt.Printf("Error happened: %s\n", err)
	}

	// add one goroutine to the wait-list
	wg.Add(1)

	//start the second node, connect it to the mining node with the enode
	go StartNode(wg, nonMiningNodeArgs(2, enode), &nodeNumber)
}

// StartNode starts an erigon node on the dev chain
func StartNode(wg *sync.WaitGroup, args []string, nodeNumber *int) {
	fmt.Printf("Arguments for node %d are: %v\n", *nodeNumber, args)

	// catch any errors and avoid panics if an error occurs
	defer func() {
		panicResult := recover()
		if panicResult == nil {
			wg.Done()
			return
		}

		log.Error("catch panic", "err", panicResult, "stack", dbg.Stack())
		wg.Done()
		os.Exit(1)
	}()

	app := erigonapp.MakeApp(runNode, erigoncli.DefaultFlags)
	*nodeNumber++ // increment the number of nodes on the network
	if err := app.Run(args); err != nil {
		_, printErr := fmt.Fprintln(os.Stderr, err)
		if printErr != nil {
			log.Warn("Error writing app run error to stderr", "err", printErr)
		}
		wg.Done()
		os.Exit(1)
	}
}

// runNode configures, creates and serves an erigon node
func runNode(ctx *cli.Context) {
	logger := log.New()

	// Initializing the node and providing the current git commit there
	logger.Info("Build info", "git_branch", params.GitBranch, "git_tag", params.GitTag, "git_commit", params.GitCommit)

	nodeCfg := node.NewNodConfigUrfave(ctx)
	ethCfg := node.NewEthConfigUrfave(ctx, nodeCfg)

	ethNode, err := node.New(nodeCfg, ethCfg, logger)
	if err != nil {
		log.Error("Devnet startup", "err", err)
		return
	}

	err = ethNode.Serve()
	if err != nil {
		log.Error("error while serving Devnet node", "err", err)
	}
}

// miningNodeArgs returns custom args for starting a mining node
func miningNodeArgs() []string {
	dataDir, _ := models.ParameterFromArgument(models.DataDirArg, "./dev")
	chainType, _ := models.ParameterFromArgument(models.ChainArg, models.ChainParam)
	devPeriod, _ := models.ParameterFromArgument(models.DevPeriodArg, models.DevPeriodParam)
	verbosity, _ := models.ParameterFromArgument(models.VerbosityArg, models.VerbosityParam)
	privateApiAddr, _ := models.ParameterFromArgument(models.PrivateApiAddrArg, models.PrivateApiParamMine)
	httpApi, _ := models.ParameterFromArgument(models.HttpApiArg, "admin,eth,erigon,web3,net,debug,trace,txpool,parity")

	return []string{models.BuildDirArg, dataDir, chainType, privateApiAddr, models.Mine, httpApi, devPeriod, verbosity}
}

// nonMiningNodeArgs returns custom args for starting a non-mining node
func nonMiningNodeArgs(nodeNumber int, enode string) []string {
	dataDir, _ := models.ParameterFromArgument(models.DataDirArg, "./dev"+fmt.Sprintf("%d", nodeNumber))
	chainType, _ := models.ParameterFromArgument(models.ChainArg, models.ChainParam)
	verbosity, _ := models.ParameterFromArgument(models.VerbosityArg, models.VerbosityParam)
	privateApiAddr, _ := models.ParameterFromArgument(models.PrivateApiAddrArg, models.PrivateApiParamNoMine)
	staticPeers, _ := models.ParameterFromArgument(models.StaticPeersArg, enode)

	return []string{models.BuildDirArg, dataDir, chainType, privateApiAddr, staticPeers, models.NoDiscover, verbosity}
}

// getEnode returns the enode of the mining node
func getEnode() (string, error) {
	nodeInfo, err := requests.AdminNodeInfo(0)
	if err != nil {
		return "", err
	}

	enode, err := utils.UniqueIDFromEnode(nodeInfo.Enode)
	if err != nil {
		return "", err
	}

	return enode, nil
}
