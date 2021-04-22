package process

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"strings"

	dataIndexer "github.com/ElrondNetwork/elastic-indexer-go/data"
	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/statistics-go/data"
	"github.com/ElrondNetwork/statistics-go/genesis"
)

const (
	epochStake = 265
)

type stakeInfoProcessor struct {
	elasticHandler  ElasticHandler
	restClient      RestClientHandler
	pubKeyConverter core.PubkeyConverter

	balances map[string]*big.Int

	accumulatedRewardDelegation *big.Int
	claimedRewards              *big.Int
	accumulatedUnJail           *big.Int

	delegationLegacyUsers map[string]*big.Int
	stakingUsers          map[string]*big.Int

	epoch uint32
	stats map[uint32]*data.StakeInfoEpoch

	delegationManagerContractAddrs []string
}

func NewStakeInfoProcessor(
	handler ElasticHandler,
	restClient RestClientHandler,
	pubKeyConverter core.PubkeyConverter,
) (*stakeInfoProcessor, error) {
	genesisAccts, err := genesis.ReadGenesisDelegationLegacyUsers("../genesis")
	if err != nil {
		return nil, err
	}

	stakingAccts, err := genesis.ReadGenesisStakingUsers("../genesis")

	return &stakeInfoProcessor{
		elasticHandler:                 handler,
		restClient:                     restClient,
		pubKeyConverter:                pubKeyConverter,
		balances:                       map[string]*big.Int{},
		accumulatedRewardDelegation:    big.NewInt(0),
		stats:                          map[uint32]*data.StakeInfoEpoch{},
		claimedRewards:                 big.NewInt(0),
		accumulatedUnJail:              big.NewInt(0),
		delegationLegacyUsers:          genesisAccts,
		stakingUsers:                   stakingAccts,
		delegationManagerContractAddrs: []string{},
	}, nil
}

func (sip *stakeInfoProcessor) ProcessEpochs() {
	for epoch := 0; epoch < epochStake; epoch++ {
		sip.epoch = uint32(epoch)
		sip.stats[sip.epoch] = &data.StakeInfoEpoch{}

		log.Printf("total staking epoch %d \n", epoch)

		err := sip.processEpoch(genesisTime+epoch*secondsInADay, genesisTime+(epoch+1)*secondsInADay)
		if err != nil {
			log.Printf("cannot proccess stake info for epoch %d, error %s", epoch, err.Error())
		}
	}

	sliceStats := make([]*data.StakeInfoEpoch, epochStake)
	for idx := uint32(0); idx < epochStake; idx++ {
		sip.stats[idx].Epoch = idx
		sliceStats[idx] = sip.stats[idx]
	}

	bytes, _ := json.MarshalIndent(sliceStats, "", " ")
	_ = ioutil.WriteFile("../reports/stakeInfoV3.json", bytes, 0644)
}

func (sip *stakeInfoProcessor) processEpoch(start, stop int) error {
	delegationBalance, err := sip.getAddressBalance(start, stop, delegationContractAddress)
	if err != nil {
		return err
	}

	stakingContractBalance, err := sip.getAddressBalance(start, stop, stakingContractAddress)
	if err != nil {
		return err
	}
	rewardTxValue, err := sip.getRewardTxValueDelegationLegacy(start, stop)
	if err != nil {
		return err
	}

	err = sip.parseTransactionsDelegationLegacyContract(start, stop)
	if err != nil {
		return err
	}

	err = sip.parseTransactionStakingContract(start, stop)
	if err != nil {
		return err
	}

	stakingContractBalanceNoJail := big.NewInt(0).Sub(stakingContractBalance, sip.accumulatedUnJail)

	sip.accumulatedRewardDelegation.Add(sip.accumulatedRewardDelegation, rewardTxValue)

	delegationBalanceNoRewards := big.NewInt(0).Sub(delegationBalance, sip.accumulatedRewardDelegation)
	delegationBalanceNoRewards.Add(delegationBalanceNoRewards, sip.claimedRewards)

	sip.stats[sip.epoch].LegacyDelegation = delegationBalanceNoRewards.String()
	sip.stats[sip.epoch].Staking = stakingContractBalanceNoJail.String()
	sip.stats[sip.epoch].TotalStaked = big.NewInt(0).Add(delegationBalanceNoRewards, stakingContractBalanceNoJail).String()
	sip.stats[sip.epoch].LegacyDelegationUser = len(sip.delegationLegacyUsers)
	sip.stats[sip.epoch].StakingUsers = len(sip.stakingUsers)

	uniquesUser := make(map[string]struct{})
	for key := range sip.delegationLegacyUsers {
		uniquesUser[key] = struct{}{}
	}
	for key := range sip.stakingUsers {
		uniquesUser[key] = struct{}{}
	}

	sip.stats[sip.epoch].TotalUniqueUsers = len(uniquesUser)

	return nil
}

func (sip *stakeInfoProcessor) getAddressBalance(start, stop int, addr string) (*big.Int, error) {
	queryDelegation := accountsHistoryAddress(start, stop, addr)
	response, err := sip.elasticHandler.DoSearchRequest(queryDelegation, accountsHistoryIndex)
	if err != nil {
		return nil, err
	}

	searchResponse := &data.SearchResponse{}
	err = json.Unmarshal(response, searchResponse)
	if err != nil {
		return nil, err
	}

	if len(searchResponse.Hits.Hits) == 0 {
		return sip.balances[addr], nil
	}

	acct := &dataIndexer.AccountBalanceHistory{}
	err = json.Unmarshal(searchResponse.Hits.Hits[0].OBJ, acct)
	if err != nil {
		return nil, err
	}

	sip.balances[addr] = stringToBigInt(acct.Balance)

	return stringToBigInt(acct.Balance), nil
}

func (sip *stakeInfoProcessor) getRewardTxValueDelegationLegacy(start, stop int) (*big.Int, error) {
	if start == genesisTime {
		return big.NewInt(0), nil
	}

	queryDelegation := rewardTxQuery(start, stop, delegationContractAddress)
	response, err := sip.elasticHandler.DoSearchRequest(queryDelegation, transactionsIndex)
	if err != nil {
		return nil, err
	}

	searchResponse := &data.SearchResponse{}
	err = json.Unmarshal(response, searchResponse)
	if err != nil {
		return nil, err
	}

	if len(searchResponse.Hits.Hits) < 1 {
		return nil, fmt.Errorf("cannot get reward transaction")
	}

	tx := &dataIndexer.Transaction{}
	err = json.Unmarshal(searchResponse.Hits.Hits[0].OBJ, tx)
	if err != nil {
		return nil, err
	}

	return stringToBigInt(tx.Value), nil
}

func (sip *stakeInfoProcessor) processAccountsHistoryResponse(responseBytes []byte) error {
	response := &data.ScrollAccountsResponse{}
	err := json.Unmarshal(responseBytes, response)
	if err != nil {
		return err
	}

	return nil
}

func (sip *stakeInfoProcessor) parseTransactionsDelegationLegacyContract(start, stop int) error {
	getTxs := getTransactionsToAddr(start, stop, delegationContractAddress)

	err := sip.elasticHandler.DoScrollRequestAllDocuments(getTxs, transactionsIndex, func(responseBytes []byte) error {
		response := &data.ScrollTransactionsSCRS{}
		errU := json.Unmarshal(responseBytes, response)
		if errU != nil {
			return errU
		}

		for _, obj := range response.Hits.Hits {
			if obj.Tx.Status != "success" {
				continue
			}

			if string(obj.Tx.Data) == "claimRewards" {
				sip.processClaimRewardsTx(&obj.Tx)
			}

			if string(obj.Tx.Data) == "stake" {
				sip.processStakeLegacy(&obj.Tx)
			}

			if strings.HasPrefix(string(obj.Tx.Data), "unStake") {
				sip.processUnStakeLegacy(&obj.Tx)
			}

			if string(obj.Tx.Data) == "unBond" {
				sip.processUnBondLegacy(&obj.Tx)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (sip *stakeInfoProcessor) processUnBondLegacy(tx *data.TxWithSCRS) {
	_, ok := sip.delegationLegacyUsers[tx.Sender]
	if !ok {
		return
	}

	if sip.delegationLegacyUsers[tx.Sender].Cmp(big.NewInt(0)) == 0 {
		delete(sip.delegationLegacyUsers, tx.Sender)
	}
}

func (sip *stakeInfoProcessor) processClaimRewardsTx(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && string(scr.Data) == "delegation rewards claim" {
			sip.claimedRewards.Add(sip.claimedRewards, stringToBigInt(scr.Value))
		}
	}
}

func (sip *stakeInfoProcessor) processStakeLegacy(tx *data.TxWithSCRS) {
	_, ok := sip.delegationLegacyUsers[tx.Sender]
	if !ok {
		sip.delegationLegacyUsers[tx.Sender] = stringToBigInt(tx.Value)
		return
	}

	sip.delegationLegacyUsers[tx.Sender].Add(sip.delegationLegacyUsers[tx.Sender], stringToBigInt(tx.Value))
}

func (sip *stakeInfoProcessor) processUnStakeLegacy(tx *data.TxWithSCRS) {
	_, ok := sip.delegationLegacyUsers[tx.Sender]
	if !ok {
		log.Printf("something is wrong get ---> this should never happend \n")
		return
	}

	spiltData := strings.Split(string(tx.Data), "@")
	if len(spiltData) < 1 {
		log.Printf("something is wrong split ---> this should never happend \n")
		return
	}

	decodedBytes, _ := hex.DecodeString(spiltData[1])

	sip.delegationLegacyUsers[tx.Sender].Sub(sip.delegationLegacyUsers[tx.Sender], big.NewInt(0).SetBytes(decodedBytes))
}

func (sip *stakeInfoProcessor) parseTransactionStakingContract(start, stop int) error {
	getTxs := getTransactionsToAddr(start, stop, stakingContractAddress)

	err := sip.elasticHandler.DoScrollRequestAllDocuments(getTxs, transactionsIndex, func(responseBytes []byte) error {
		response := &data.ScrollTransactionsSCRS{}
		errU := json.Unmarshal(responseBytes, response)
		if errU != nil {
			return errU
		}

		for _, obj := range response.Hits.Hits {
			if obj.Tx.Status != "success" {
				continue
			}

			if string(obj.Tx.Data) == "unJail" {
				sip.accumulatedUnJail.Add(sip.accumulatedUnJail, stringToBigInt(obj.Tx.Value))
			}

			if strings.HasPrefix(string(obj.Tx.Data), "stake") || string(obj.Tx.Data) == "stake" {
				sip.processStakeTx(&obj.Tx)
			}
			if strings.HasPrefix(string(obj.Tx.Data), "unBond") || string(obj.Tx.Data) == "claim" || string(obj.Tx.Data) == "unBondTokens" {
				sip.processUnBond(&obj.Tx)
			}

		}

		return nil
	})
	if err != nil {
		return err
	}

	if sip.epoch < 239 {
		return nil
	}

	if sip.epoch == 239 {
		sip.delegationManagerContractAddrs, err = sip.getAllDelegationManagerContracts()
		if err != nil {
			return err
		}
	}

	for _, contractAddr := range sip.delegationManagerContractAddrs {
		getTxsDelegation := getTransactionsToAddr(start, stop, contractAddr)

		errSCR := sip.elasticHandler.DoScrollRequestAllDocuments(getTxsDelegation, transactionsIndex, func(responseBytes []byte) error {
			response := &data.ScrollTransactionsSCRS{}
			errU := json.Unmarshal(responseBytes, response)
			if errU != nil {
				return errU
			}

			for _, obj := range response.Hits.Hits {
				if obj.Tx.Status != "success" {
					continue
				}

				if string(obj.Tx.Data) == "delegate" {
					sip.processDelegationManagerDelegate(&obj.Tx)
				}
				if string(obj.Tx.Data) == "withdraw" {
					sip.processDelegationManagerWithdraw(&obj.Tx)
				}
				if string(obj.Tx.Data) == "reDelegateRewards" {
					sip.processDelegationManagerRedelegate(&obj.Tx)
				}

			}

			return nil
		})
		if errSCR != nil {
			return err
		}
	}

	return nil
}

func (sip *stakeInfoProcessor) processDelegationManagerDelegate(tx *data.TxWithSCRS) {
	_, ok := sip.stakingUsers[tx.Sender]
	if !ok {
		sip.stakingUsers[tx.Sender] = stringToBigInt(tx.Value)
		return
	}

	sip.stakingUsers[tx.Sender].Add(sip.stakingUsers[tx.Sender], stringToBigInt(tx.Value))
}

func (sip *stakeInfoProcessor) processDelegationManagerWithdraw(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && tx.Sender == scr.Receiver {
			_, ok := sip.stakingUsers[tx.Sender]
			if !ok {
				log.Printf("something is wrong get staking user from map Withdraw ---> this should never happend \n")
				return
			}

			sip.stakingUsers[tx.Sender].Sub(sip.stakingUsers[tx.Sender], stringToBigInt(scr.Value))
			if sip.stakingUsers[tx.Sender].Cmp(big.NewInt(0)) == 0 {
				delete(sip.stakingUsers, tx.Sender)
			}
			return
		}
	}
}
func (sip *stakeInfoProcessor) processDelegationManagerRedelegate(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && scr.Receiver == stakingContractAddress {
			_, ok := sip.stakingUsers[tx.Sender]
			if !ok {
				log.Printf("something is wrong get staking user from map Redelegate ---> this should never happend \n")
				return
			}

			sip.stakingUsers[tx.Sender].Add(sip.stakingUsers[tx.Sender], stringToBigInt(scr.Value))
		}
	}
}

func (sip *stakeInfoProcessor) processStakeTx(tx *data.TxWithSCRS) {
	_, ok := sip.stakingUsers[tx.Sender]
	if !ok {
		sip.stakingUsers[tx.Sender] = stringToBigInt(tx.Value)
		return
	}

	sip.stakingUsers[tx.Sender].Add(sip.stakingUsers[tx.Sender], stringToBigInt(tx.Value))
}

func (sip *stakeInfoProcessor) processUnBond(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && tx.Sender == scr.Receiver {
			_, ok := sip.stakingUsers[tx.Sender]
			if !ok {
				log.Printf("something is wrong get staking user from map UnBond ---> this should never happend \n")
				return
			}

			sip.stakingUsers[tx.Sender].Sub(sip.stakingUsers[tx.Sender], stringToBigInt(scr.Value))
			if sip.stakingUsers[tx.Sender].Cmp(big.NewInt(0)) == 0 {
				delete(sip.stakingUsers, tx.Sender)
			}
			return
		}
	}
}

func (sip *stakeInfoProcessor) getAllDelegationManagerContracts() ([]string, error) {
	vmRequest := &data.VmValueRequest{
		Address:    "erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqylllslmq6y6",
		FuncName:   "getAllContractAddresses",
		CallerAddr: "erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqylllslmq6y6",
	}

	responseVmValue := &data.ResponseVmValue{}
	err := sip.restClient.CallPostRestEndPoint("/vm-values/query", vmRequest, responseVmValue)
	if err != nil {
		return nil, err
	}
	if responseVmValue.Error != "" {
		return nil, fmt.Errorf("%s", responseVmValue.Error)
	}

	returnedData := responseVmValue.Data.Data.ReturnData
	encodedAddrs := make([]string, 0)
	for _, addr := range returnedData {
		encodedAddrs = append(encodedAddrs, sip.pubKeyConverter.Encode(addr))
	}

	return encodedAddrs, nil
}
