package process

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	dataIndexer "github.com/ElrondNetwork/elastic-indexer-go/data"
	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/statistics-go/data"
	"github.com/ElrondNetwork/statistics-go/genesis"
)

const (
	delegationManager = "erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqylllslmq6y6"
)

type stakeInfoProcessor struct {
	elasticHandler                 ElasticHandler
	restClient                     RestClientHandler
	pubKeyConverter                core.PubkeyConverter
	accumulatedRewardDelegation    *big.Int
	claimedRewards                 *big.Int
	accumulatedUnJail              *big.Int
	stats                          map[uint32]*data.StakeInfoEpoch
	delegationLegacyUsers          map[string]*big.Int
	stakingUsers                   map[string]*big.Int
	balances                       map[string]*big.Int
	delegationManagerContractAddrs []string
	delegationContractAddress      string
	stakingContractAddress         string
	epoch                          uint32
	genesisTime                    int

	delegatorDelegationManager map[string]*big.Int
}

func NewStakeInfoProcessor(
	handler ElasticHandler,
	restClient RestClientHandler,
	pubKeyConverter core.PubkeyConverter,
	pathGenesisFiles string,
	genesisTime int,
	delegationContractAddress string,
	stakingContractAddress string,
) (*stakeInfoProcessor, error) {
	genesisAccts, err := genesis.ReadGenesisDelegationLegacyUsers(pathGenesisFiles)
	if err != nil {
		return nil, err
	}

	stakingAccts, err := genesis.ReadGenesisStakingUsers("../genesis")

	return &stakeInfoProcessor{
		elasticHandler:                 handler,
		restClient:                     restClient,
		pubKeyConverter:                pubKeyConverter,
		genesisTime:                    genesisTime,
		balances:                       map[string]*big.Int{},
		accumulatedRewardDelegation:    big.NewInt(0),
		stats:                          map[uint32]*data.StakeInfoEpoch{},
		claimedRewards:                 big.NewInt(0),
		accumulatedUnJail:              big.NewInt(0),
		delegationLegacyUsers:          genesisAccts,
		stakingUsers:                   stakingAccts,
		delegationManagerContractAddrs: []string{},
		delegationContractAddress:      delegationContractAddress,
		stakingContractAddress:         stakingContractAddress,
		delegatorDelegationManager:     map[string]*big.Int{},
	}, nil
}

func (sip *stakeInfoProcessor) ProcessEpochs(endEpoch uint32) ([]byte, error) {
	for epoch := uint32(0); epoch < endEpoch; epoch++ {
		sip.epoch = epoch
		sip.stats[sip.epoch] = &data.StakeInfoEpoch{}

		log.Printf("total staking epoch %d \n", epoch)

		err := sip.processEpoch(sip.genesisTime+int(epoch)*secondsInADay, sip.genesisTime+int(epoch+1)*secondsInADay)
		if err != nil {
			log.Printf("cannot proccess stake info for epoch %d, error %s", epoch, err.Error())
		}
	}

	sliceStats := make([]*data.StakeInfoEpoch, endEpoch)
	for idx := uint32(0); idx < endEpoch; idx++ {
		sip.stats[idx].Epoch = idx
		sliceStats[idx] = sip.stats[idx]
	}

	bytes, _ := json.MarshalIndent(sliceStats, "", " ")
	return bytes, nil
}

func (sip *stakeInfoProcessor) processEpoch(start, stop int) error {
	delegationBalance, err := sip.getAddressBalance(start, stop, sip.delegationContractAddress)
	if err != nil {
		return err
	}

	stakingContractBalance, err := sip.getAddressBalance(start, stop, sip.stakingContractAddress)
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
	// totalStakeDelegationLegacy := big.NewInt(0)
	for key := range sip.delegationLegacyUsers {
		uniquesUser[key] = struct{}{}
		// totalStakeDelegationLegacy.Add(totalStakeDelegationLegacy, value)
	}
	// sip.stats[sip.epoch].TotalDelegatedLegacy = totalStakeDelegationLegacy.String()

	activeLegacyDelegation, _ := big.NewInt(0).SetString("3650000000000000000000000", 10)
	sip.stats[sip.epoch].TotalDelegatedLegacy = big.NewInt(0).Add(activeLegacyDelegation, delegationBalanceNoRewards).String()

	for key := range sip.stakingUsers {
		uniquesUser[key] = struct{}{}
	}

	sip.stats[sip.epoch].TotalUniqueUsers = len(uniquesUser)

	sip.stats[sip.epoch].DelegationUsers = len(sip.delegatorDelegationManager)
	delegationStake := big.NewInt(0)
	for _, value := range sip.delegatorDelegationManager {
		delegationStake.Add(delegationStake, value)
	}

	sip.stats[sip.epoch].Delegation = delegationStake.String()

	DumpBalances(sip.delegationLegacyUsers, sip.stakingUsers, "../reportsV2/balances", sip.epoch)
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
	if start == sip.genesisTime {
		return big.NewInt(0), nil
	}

	queryDelegation := rewardTxQuery(start, stop, sip.delegationContractAddress)
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
	getTxs := getTransactionsToAddr(start, stop, sip.delegationContractAddress)

	err := sip.elasticHandler.DoScrollRequestAllDocuments(getTxs, transactionsIndex, func(responseBytes []byte) error {
		response := &data.ScrollTransactionsSCRS{}
		errU := json.Unmarshal(responseBytes, response)
		if errU != nil {
			return errU
		}

		for _, obj := range response.Hits.Hits {
			if obj.Tx.Status != "success" || obj.Tx.Sender == "4294967295" {
				continue
			}

			obj.Tx.Hash = obj.ID

			if string(obj.Tx.Data) == "claimRewards" {
				sip.processClaimRewardsTx(&obj.Tx)
				continue
			}

			if string(obj.Tx.Data) == "stake" {
				sip.processStakeLegacy(&obj.Tx)
				continue
			}

			if strings.HasPrefix(string(obj.Tx.Data), "unStake@") {
				sip.processUnStakeLegacy(&obj.Tx)
				continue
			}

			if string(obj.Tx.Data) == "unBond" {
				sip.processUnBondLegacy(&obj.Tx)
				continue
			}

			continue
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

	// for _, scr := range tx.SCRS {
	// 	if scr.Nonce == 0 && scr.Value != "" && string(scr.Data) == "delegation stake unbond" && scr.Receiver == tx.Sender {
	// 		sip.delegationLegacyUsers[tx.Sender].Sub(sip.delegationLegacyUsers[tx.Sender], stringToBigInt(scr.Value))
	// 		if sip.delegationLegacyUsers[tx.Sender].Cmp(big.NewInt(0)) == 0 {
	// 			delete(sip.delegationLegacyUsers, tx.Sender)
	// 		}
	// 		return
	// 	}
	// }

	if sip.delegationLegacyUsers[tx.Sender].Cmp(big.NewInt(0)) == 0 {
		delete(sip.delegationLegacyUsers, tx.Sender)
	}
}

func (sip *stakeInfoProcessor) processClaimRewardsTx(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && string(scr.Data) == "delegation rewards claim" {
			sip.claimedRewards.Add(sip.claimedRewards, stringToBigInt(scr.Value))
			return
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
		log.Printf("transaction hash %s", tx.Hash)
		log.Printf("something is wrong get UnStake ---> this should never happend \n")
		return
	}

	spiltData := strings.Split(string(tx.Data), "@")
	if len(spiltData) < 1 {
		log.Printf("something is wrong split ---> this should never happend \n")
		return
	}

	decodedBytes, _ := hex.DecodeString(spiltData[1])
	bigValue := big.NewInt(0).SetBytes(decodedBytes)

	sip.delegationLegacyUsers[tx.Sender].Sub(sip.delegationLegacyUsers[tx.Sender], bigValue)

	if sip.delegationLegacyUsers[tx.Sender].Cmp(big.NewInt(0)) < 0 {
		log.Printf("something is wrong get negative value ---> this should never happend \n")

		// sip.delegationLegacyUsers[tx.Sender] = big.NewInt(0)
		delete(sip.delegationLegacyUsers, tx.Sender)
	}
}

func (sip *stakeInfoProcessor) parseTransactionStakingContract(start, stop int) error {
	getTxs := getTransactionsToAddr(start, stop, sip.stakingContractAddress)

	err := sip.elasticHandler.DoScrollRequestAllDocuments(getTxs, transactionsIndex, func(responseBytes []byte) error {
		response := &data.ScrollTransactionsSCRS{}
		errU := json.Unmarshal(responseBytes, response)
		if errU != nil {
			return errU
		}

		for _, obj := range response.Hits.Hits {
			if obj.Tx.Status != "success" ||
				strings.HasPrefix(string(obj.Tx.Data), "changeRewardAddress") ||
				strings.HasPrefix(string(obj.Tx.Data), "unStake") {
				continue
			}

			if strings.HasPrefix(string(obj.Tx.Data), "unJail") {
				sip.accumulatedUnJail.Add(sip.accumulatedUnJail, stringToBigInt(obj.Tx.Value))
				continue
			}

			if strings.HasPrefix(string(obj.Tx.Data), "stake") || string(obj.Tx.Data) == "stake" {
				sip.processStakeTx(&obj.Tx)
				continue
			}
			if strings.HasPrefix(string(obj.Tx.Data), "unBond") || string(obj.Tx.Data) == "claim" || string(obj.Tx.Data) == "unBondTokens" {
				sip.processUnBond(&obj.Tx)
				continue
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

	sip.delegationManagerContractAddrs, err = sip.getAllDelegationManagerContracts()
	if err != nil {
		return err
	}

	err = sip.processTxsToDelegationManagerCreator(start, stop)
	if err != nil {
		return err
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

				obj.Tx.Hash = obj.ID

				if string(obj.Tx.Data) == "delegate" {
					sip.processDelegationManagerDelegate(&obj.Tx)
					continue
				}
				if string(obj.Tx.Data) == "withdraw" {
					sip.processDelegationManagerWithdraw(&obj.Tx)
					continue
				}
				if string(obj.Tx.Data) == "reDelegateRewards" {
					sip.processDelegationManagerReDelegate(&obj.Tx)
					continue
				}

				continue

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
		goto STEP2
	}

	sip.stakingUsers[tx.Sender].Add(sip.stakingUsers[tx.Sender], stringToBigInt(tx.Value))

STEP2:
	_, ok = sip.delegatorDelegationManager[tx.Sender]
	if !ok {
		sip.delegatorDelegationManager[tx.Sender] = stringToBigInt(tx.Value)
		return
	}

	sip.delegatorDelegationManager[tx.Sender].Add(sip.delegatorDelegationManager[tx.Sender], stringToBigInt(tx.Value))
}

func (sip *stakeInfoProcessor) processDelegationManagerWithdraw(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && tx.Sender == scr.Receiver {
			_, ok := sip.stakingUsers[tx.Sender]
			if !ok {
				log.Printf("something is wrong get staking user from map Withdraw ---> this should never happend \n")
				goto STEP2
			}

			sip.stakingUsers[tx.Sender].Sub(sip.stakingUsers[tx.Sender], stringToBigInt(scr.Value))
			if sip.stakingUsers[tx.Sender].Cmp(big.NewInt(0)) == 0 {
				delete(sip.stakingUsers, tx.Sender)
			}

		STEP2:
			_, ok = sip.delegatorDelegationManager[tx.Sender]
			if !ok {
				log.Printf("something is wrong get staking user from map Withdraw delegatorDelegationManager ---> this should never happend \n")
				return
			}
			sip.delegatorDelegationManager[tx.Sender].Sub(sip.delegatorDelegationManager[tx.Sender], stringToBigInt(scr.Value))
			if sip.delegatorDelegationManager[tx.Sender].Cmp(big.NewInt(0)) == 0 {
				delete(sip.delegatorDelegationManager, tx.Sender)
			}
			return
		}
	}
}
func (sip *stakeInfoProcessor) processDelegationManagerReDelegate(tx *data.TxWithSCRS) {
	for _, scr := range tx.SCRS {
		if scr.Nonce == 0 && scr.Value != "" && scr.Receiver == sip.stakingContractAddress {
			_, ok := sip.stakingUsers[tx.Sender]
			if !ok {
				log.Printf("transaction hash %s", tx.Hash)
				log.Printf("something is wrong get staking user from map Redelegate ---> this should never happend \n")
				goto STEP2
			}

			sip.stakingUsers[tx.Sender].Add(sip.stakingUsers[tx.Sender], stringToBigInt(scr.Value))

		STEP2:
			_, ok = sip.delegatorDelegationManager[tx.Sender]
			if !ok {
				log.Printf("something is wrong get staking user from map Redelegate delegatorDelegationManager---> this should never happend \n")
				return
			}

			sip.delegatorDelegationManager[tx.Sender].Add(sip.delegatorDelegationManager[tx.Sender], stringToBigInt(scr.Value))
			return
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
		Address:    delegationManager,
		FuncName:   "getAllContractAddresses",
		CallerAddr: delegationManager,
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

func (sip *stakeInfoProcessor) processTxsToDelegationManagerCreator(start, stop int) error {
	getTxs := getTransactionsToAddr(start, stop, delegationManager)

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

			if strings.HasPrefix(string(obj.Tx.Data), "createNewDelegationContract@") {
				sip.processDelegationManagerDelegate(&obj.Tx)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
