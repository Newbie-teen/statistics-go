package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/statistics-go/config"
	"github.com/ElrondNetwork/statistics-go/statistics"
	"github.com/urfave/cli"
)

const (
	optionTxs      = "transactions"
	optionAccounts = "accounts"
	optionStake    = "stake"
)

var (
	// configurationFile defines a flag for the path to the main toml configuration file
	configurationFile = cli.StringFlag{
		Name:  "config",
		Usage: "The main configuration file to load",
		Value: "./config/config.toml",
	}
	genesisFolder = cli.StringFlag{
		Name:  "path-genesis-folder",
		Usage: "The path to the folder with the genesis files",
		Value: "../genesis",
	}
	endEpoch = cli.IntFlag{
		Name:  "end-epoch",
		Usage: "The epoch until statistics are generated",
		Value: 0,
	}
	generateStatsOptions = cli.StringFlag{
		Name:  "stats",
		Usage: "Will generate statistics about transactions, accounts or stake",
		Value: "accounts",
	}
	outputFile = cli.StringFlag{
		Name:  "output-file",
		Usage: "The output file with statistics",
		Value: "output.json",
	}
)

func main() {
	app := cli.NewApp()

	app.Name = "Elrond Statistics GO"
	app.Version = "v1.0.0"
	app.Usage = "This is the entry point for starting a new Elrond Statistics GO"
	app.Flags = []cli.Flag{
		configurationFile,
		genesisFolder,
		endEpoch,
		generateStatsOptions,
		outputFile,
	}
	app.Authors = []cli.Author{
		{
			Name:  "The Elrond Team",
			Email: "contact@elrond.com",
		},
	}

	app.Action = startStatistics

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func startStatistics(ctx *cli.Context) error {
	configurationFileName := ctx.GlobalString(configurationFile.Name)
	pathGenesisFile := ctx.GlobalString(genesisFolder.Name)
	statsOption := ctx.GlobalString(generateStatsOptions.Name)
	endEpochV := ctx.GlobalInt(endEpoch.Name)
	outputFileV := ctx.GlobalString(outputFile.Name)

	generalConfig, err := loadMainConfig(configurationFileName)
	if err != nil {
		return err
	}

	statsHandler, err := statistics.CreateStatsHandler(generalConfig, pathGenesisFile)
	if err != nil {
		return err
	}

	var bytes []byte
	switch statsOption {
	case optionAccounts:
		bytes, err = statsHandler.ProcessAllAccounts(uint32(endEpochV))
	case optionStake:
		bytes, err = statsHandler.ProcessStakeInfo(uint32(endEpochV))
	case optionTxs:
		bytes, err = statsHandler.ProcessAllTransactions(uint32(endEpochV))
	default:
		return fmt.Errorf("please provide a valid option: %s, %s, %s", optionAccounts, optionStake, optionTxs)
	}

	if err != nil {
		return err
	}

	_ = ioutil.WriteFile(outputFileV, bytes, 0644)

	return nil
}

func loadMainConfig(filepath string) (*config.Config, error) {
	cfg := &config.Config{}
	err := core.LoadTomlFile(cfg, filepath)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
