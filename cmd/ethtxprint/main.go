package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/juztin/ethtxprint"
)

func main() {
	nodeURLFlag := flag.String("node", "http://localhost:8545", "Ethereum node URL")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("Must supply a single transaction has argument")
		os.Exit(1)
	}
	hash := common.HexToHash(args[0])
	if len(hash) != 32 {
		fmt.Println("Invalid transaction hash provided")
		os.Exit(1)
	}
	c, err := ethclient.Dial(*nodeURLFlag)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	tx, err := ethtxprint.NewTransaction(context.Background(), c, hash)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(tx)
}
