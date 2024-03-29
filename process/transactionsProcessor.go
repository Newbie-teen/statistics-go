package process

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	dataIndexer "github.com/ElrondNetwork/elastic-indexer-go/data"
	"github.com/ElrondNetwork/elrond-go/core"
	dataTx "github.com/ElrondNetwork/elrond-go/data/transaction"
	"github.com/ElrondNetwork/statistics-go/data"
	"github.com/ElrondNetwork/statistics-go/genesis"
)

const (
	secondsInADay = 24 * 3600
)

type transactionsProc struct {
	pubKeyConverter core.PubkeyConverter
	elasticHandler  ElasticHandler
	genesisTime     int
	addresses       map[string]struct{}
	stats           map[uint32]*data.StatisticsEpoch
	epoch           uint32

	dailyActiveAccounts  map[string]int
	dailyActiveContracts map[string]int
}

func NewTransactionsProcessor(
	elasticHandler ElasticHandler,
	pubKeyConverter core.PubkeyConverter,
	pathToGenesisFiles string,
	genesisTime int,
) (*transactionsProc, error) {
	addresses, err := genesis.ReadGenesisAddresses(pathToGenesisFiles)
	if err != nil {
		return nil, err
	}

	return &transactionsProc{
		pubKeyConverter:      pubKeyConverter,
		elasticHandler:       elasticHandler,
		addresses:            addresses,
		stats:                map[uint32]*data.StatisticsEpoch{},
		epoch:                0,
		dailyActiveContracts: make(map[string]int),
		dailyActiveAccounts:  make(map[string]int),
		genesisTime:          genesisTime,
	}, nil
}

func (tp *transactionsProc) ProcessAllTxs(endEpoch uint32) ([]byte, error) {
	for epoch := uint32(0); epoch < endEpoch; epoch++ {
		log.Printf("process transactions epoch %d \n", epoch)
		tp.epoch = epoch
		tp.stats[tp.epoch] = &data.StatisticsEpoch{}

		err := tp.processTransactionsEpoch(tp.genesisTime+int(epoch)*secondsInADay, tp.genesisTime+int(epoch+1)*secondsInADay)
		if err != nil {
			log.Printf("process transaction epoch %d, error: %s", epoch, err.Error())
		}
	}

	sliceStats := make([]*data.StatisticsEpoch, endEpoch)
	for idx := uint32(0); idx < endEpoch; idx++ {
		tp.stats[idx].Epoch = idx
		sliceStats[idx] = tp.stats[idx]
	}

	bytes, _ := json.MarshalIndent(sliceStats, "", " ")

	return bytes, nil
}

func (tp *transactionsProc) processTransactionsEpoch(startTime, endTime int) error {
	defer func() {
		tp.dailyActiveAccounts = make(map[string]int)
		tp.dailyActiveContracts = make(map[string]int)
	}()

	err := tp.elasticHandler.DoScrollRequestAllDocuments(getTransactionsByTimestamp(startTime, endTime), "transactions", tp.processTransactionsResponse)
	if err != nil {
		return err
	}

	tp.stats[tp.epoch].SetInfoAboutDailyAccounts(tp.dailyActiveAccounts)
	tp.stats[tp.epoch].SetInfoAboutDailyContracts(tp.dailyActiveContracts)

	return nil
}

func (tp *transactionsProc) processTransactionsResponse(responseBytes []byte) error {
	txsResponse := data.ScrollTransactionsResponse{}
	err := json.Unmarshal(responseBytes, &txsResponse)
	if err != nil {
		return err
	}

	for _, txRes := range txsResponse.Hits.Hits {
		tp.setMetricForATx(txRes.Tx)
		tp.checkRelayedTx(txRes.Tx)
	}

	return nil
}

func (tp *transactionsProc) setMetricForATx(tx dataIndexer.Transaction) {
	if tx.Sender != fmt.Sprintf("%d", core.MetachainShardId) {
		tp.dailyActiveAccounts[tx.Sender]++
	}

	decodedReceiver, _ := tp.pubKeyConverter.Decode(tx.Receiver)
	isSCAddr := core.IsSmartContractAddress(decodedReceiver)
	if isSCAddr {
		tp.dailyActiveContracts[tx.Receiver]++
		tp.stats[tp.epoch].DailyContractCalls++
	}

	tp.stats[tp.epoch].DailyTransactions++

	if tx.Sender == fmt.Sprintf("%d", core.MetachainShardId) {
		return
	}

	_, exists := tp.addresses[tx.Sender]
	if !exists {
		tp.addresses[tx.Sender] = struct{}{}

		tp.stats[tp.epoch].DailyNewAddresses++
	}

	_, exists = tp.addresses[tx.Receiver]
	if !exists {
		tp.addresses[tx.Receiver] = struct{}{}

		tp.stats[tp.epoch].DailyNewAddresses++

		if isSCAddr {
			tp.stats[tp.epoch].DailyNewContractAddresses++
		}
	}
}

func (tp *transactionsProc) checkRelayedTx(txx dataIndexer.Transaction) {
	txData := string(txx.Data)
	if txData == "" {
		return
	}
	if txx.Status == "fail" {
		return
	}

	if !strings.HasPrefix(txData, core.RelayedTransaction) {
		return
	}

	splitData := strings.Split(txData, "@")
	if len(splitData) < 2 {
		return
	}

	innerTx := &dataTx.Transaction{}
	err := json.Unmarshal([]byte(splitData[1]), innerTx)
	if err != nil {
		return
	}

	senderBech32 := tp.pubKeyConverter.Encode(innerTx.SndAddr)
	receiverBech32 := tp.pubKeyConverter.Encode(innerTx.RcvAddr)

	tp.dailyActiveAccounts[senderBech32]++

	isSCAddr := core.IsSmartContractAddress(innerTx.RcvAddr)
	if isSCAddr {
		tp.dailyActiveContracts[receiverBech32]++
		tp.stats[tp.epoch].DailyContractCalls++
	}

	_, exists := tp.addresses[receiverBech32]
	if !exists {
		tp.addresses[receiverBech32] = struct{}{}

		tp.stats[tp.epoch].DailyNewAddresses++

		if isSCAddr {
			tp.stats[tp.epoch].DailyNewContractAddresses++
		}
	}
}
