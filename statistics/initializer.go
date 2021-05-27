package statistics

import (
	"fmt"

	"github.com/ElrondNetwork/elrond-go/data/state/factory"
	"github.com/ElrondNetwork/statistics-go/config"
	"github.com/ElrondNetwork/statistics-go/data"
	"github.com/ElrondNetwork/statistics-go/elasticClient"
	"github.com/ElrondNetwork/statistics-go/process"
	"github.com/ElrondNetwork/statistics-go/restClient"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/tidwall/gjson"
)

func CreateStatsHandler(cfg *config.Config, pathGenesisFiles string) (StatsHandler, error) {
	elasticCfg := elasticsearch.Config{
		Addresses: []string{cfg.GeneralConfig.ElasticDatabaseAddress},
		Username:  cfg.GeneralConfig.Username,
		Password:  cfg.GeneralConfig.Password,
	}
	esClient, err := elasticClient.NewElasticClient(elasticCfg)
	if err != nil {
		return nil, err
	}

	rClient, err := restClient.NewRestClient(cfg.GeneralConfig.APIUrl)
	if err != nil {
		return nil, err
	}

	pubKeyConverter, err := factory.NewPubkeyConverter(cfg.AddressPubkeyConverter)
	if err != nil {
		return nil, err
	}

	genesisTime, err := fetchGenesisTime(rClient)
	if err != nil {
		return nil, err
	}

	acctsHandler, err := process.NewAccountsProcessor(esClient, pubKeyConverter, genesisTime)
	if err != nil {
		return nil, err
	}

	stakeInfoHandler, err := process.NewStakeInfoProcessor(
		esClient,
		rClient,
		pubKeyConverter,
		pathGenesisFiles,
		genesisTime,
		cfg.GeneralConfig.DelegationLegacyContractAddress,
		cfg.GeneralConfig.StakingContractAddress,
	)
	if err != nil {
		return nil, err
	}

	transactionsHandler, err := process.NewTransactionsProcessor(esClient, pubKeyConverter, pathGenesisFiles, genesisTime)
	if err != nil {
		return nil, err
	}

	return process.NewStatisticsProcessor(transactionsHandler, acctsHandler, stakeInfoHandler)
}

func fetchGenesisTime(rClient process.RestClientHandler) (int, error) {
	genericAPIResponse := &data.GenericAPIResponse{}
	err := rClient.CallGetRestEndPoint("/network/config", genericAPIResponse)
	if err != nil {
		return 0, err
	}
	if genericAPIResponse.Error != "" {
		return 0, fmt.Errorf("%s", genericAPIResponse.Error)
	}

	genesisTime := gjson.Get(string(genericAPIResponse.Data), "config.erd_start_time")
	if genesisTime.String() == "" || genesisTime.String() == "0" {
		return 0, fmt.Errorf("%s", "cannot fecth genesis timestamp")
	}

	return int(genesisTime.Num), nil
}
