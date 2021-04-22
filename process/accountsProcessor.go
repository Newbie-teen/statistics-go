package process

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/big"
	"time"

	dataIndexer "github.com/ElrondNetwork/elastic-indexer-go/data"
	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/statistics-go/data"
)

const (
	epochAccounts = 1
)

var (
	nonZero     = big.NewInt(0)
	b01EGLD, _  = big.NewInt(0).SetString("100000000000000000", 10)
	b1EGLD, _   = big.NewInt(0).SetString("1000000000000000000", 10)
	b10EGLD, _  = big.NewInt(0).SetString("10000000000000000000", 10)
	b100EGLD, _ = big.NewInt(0).SetString("100000000000000000000", 10)
	b1KEGLD, _  = big.NewInt(0).SetString("1000000000000000000000", 10)
)

type accountInfo struct {
	balance   *big.Int
	timestamp time.Duration
}

type accountsProcessor struct {
	elasticHandler  ElasticHandler
	stats           map[uint32]*data.StatisticsAddressesBalanceEpoch
	accounts        map[string]*accountInfo
	epoch           uint32
	totalContract   int
	pubKeyConverter core.PubkeyConverter
}

func NewAccountsProcessor(
	elasticHandler ElasticHandler,
	pubKeyConverter core.PubkeyConverter,
) (*accountsProcessor, error) {
	return &accountsProcessor{
		elasticHandler:  elasticHandler,
		pubKeyConverter: pubKeyConverter,
		stats:           map[uint32]*data.StatisticsAddressesBalanceEpoch{},
		accounts:        map[string]*accountInfo{},
	}, nil
}

func (ap *accountsProcessor) ProcessAllAccounts() {
	for epoch := 0; epoch < epochAccounts; epoch++ {
		log.Printf("process accounts history epoch %d \n", epoch)

		ap.epoch = uint32(epoch)
		ap.stats[ap.epoch] = &data.StatisticsAddressesBalanceEpoch{}

		err := ap.processAccountsEpoch(genesisTime+epoch*secondsInADay, genesisTime+(epoch+1)*secondsInADay)
		if err != nil {
			log.Printf("cannot proccess accouts for epoch %d, error %s", epoch, err.Error())
			continue
		}

		ap.setCounts()
		ap.stats[ap.epoch].TotalAddresses = len(ap.accounts)
		ap.stats[ap.epoch].TotalContractAddresses = ap.totalContract
	}

	sliceStats := make([]*data.StatisticsAddressesBalanceEpoch, epochAccounts)
	for idx := uint32(0); idx < epochAccounts; idx++ {
		ap.stats[idx].Epoch = idx
		sliceStats[idx] = ap.stats[idx]
	}

	bytes, _ := json.MarshalIndent(sliceStats, "", " ")

	_ = ioutil.WriteFile("../reports/statsAccountsV4.json", bytes, 0644)
}

func (ap *accountsProcessor) processAccountsEpoch(start, stop int) error {
	err := ap.elasticHandler.DoScrollRequestAllDocuments(getTransactionsByTimestamp(start, stop), "accountshistory", ap.processAccountsHistoryResponse)
	if err != nil {
		return err
	}

	return nil
}

func (ap *accountsProcessor) processAccountsHistoryResponse(responseBytes []byte) error {
	response := &data.ScrollAccountsResponse{}
	err := json.Unmarshal(responseBytes, response)
	if err != nil {
		return err
	}

	for _, acctResponse := range response.Hits.Hits {
		ap.extractAccountInfo(&acctResponse.Account)
	}

	return nil
}

func (ap *accountsProcessor) extractAccountInfo(acct *dataIndexer.AccountBalanceHistory) {
	_, ok := ap.accounts[acct.Address]
	if !ok {
		ap.accounts[acct.Address] = &accountInfo{
			balance:   stringToBigInt(acct.Balance),
			timestamp: acct.Timestamp,
		}
	}

	addrDecoded, _ := ap.pubKeyConverter.Decode(acct.Address)
	if !ok && core.IsSmartContractAddress(addrDecoded) {
		ap.totalContract++
	}

	if ap.accounts[acct.Address].timestamp > acct.Timestamp {
		return
	}

	ap.accounts[acct.Address].timestamp = acct.Timestamp
	ap.accounts[acct.Address].balance = stringToBigInt(acct.Balance)

	return
}

func (ap *accountsProcessor) setCounts() {
	currentEpochStats := ap.stats[ap.epoch]

	for _, acctInfo := range ap.accounts {

		if acctInfo.balance.Cmp(nonZero) > 0 {
			currentEpochStats.NonZero++
		}

		if acctInfo.balance.Cmp(b01EGLD) >= 0 {
			currentEpochStats.B01EGLD++
		}

		if acctInfo.balance.Cmp(b1EGLD) >= 0 {
			currentEpochStats.B1EGLD++
		}

		if acctInfo.balance.Cmp(b10EGLD) >= 0 {
			currentEpochStats.B10EGLD++
		}

		if acctInfo.balance.Cmp(b100EGLD) >= 0 {
			currentEpochStats.B100EGLD++
		}

		if acctInfo.balance.Cmp(b1KEGLD) >= 0 {
			currentEpochStats.B1kEGLD++
		}
	}
}

func stringToBigInt(b string) *big.Int {
	bigV, ok := big.NewInt(0).SetString(b, 10)
	if !ok {
		return big.NewInt(0)
	}

	return bigV
}
