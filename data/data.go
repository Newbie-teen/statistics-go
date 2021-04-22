package data

import (
	"encoding/json"

	"github.com/ElrondNetwork/elastic-indexer-go/data"
	"github.com/ElrondNetwork/elrond-go/data/vm"
)

type ScrollTransactionsResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []struct {
			ID string           `json:"_id"`
			Tx data.Transaction `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type StatisticsEpoch struct {
	Epoch                       uint32         `json:"epoch"`
	DailyTransactions           int            `json:"dailyTransactions"`
	DailyContractCalls          int            `json:"dailyContractCalls"`
	DailyActiveAccounts         int            `json:"dailyActiveAccounts"`
	DailyActiveContractAccounts int            `json:"dailyActiveContractAccounts"`
	DailyNewAddresses           int            `json:"dailyNewAddresses"`
	DailyNewContractAddresses   int            `json:"dailyNewContractAddresses"`
	TopActiveAddresses          map[string]int `json:"topActiveAccounts"`
	TopActiveContracts          map[string]int `json:"topActiveContracts"`
}

func (se *StatisticsEpoch) SetInfoAboutDailyAccounts(dailyAccounts map[string]int) {
	se.TopActiveAddresses = make(map[string]int)

	se.DailyActiveAccounts = len(dailyAccounts)
	for key, value := range dailyAccounts {
		se.TopActiveAddresses[key] = value
	}
}

func (se *StatisticsEpoch) SetInfoAboutDailyContracts(dailyContracts map[string]int) {
	se.TopActiveContracts = make(map[string]int)

	se.DailyActiveContractAccounts = len(dailyContracts)
	for key, value := range dailyContracts {
		se.TopActiveContracts[key] = value
	}
}

type ScrollAccountsResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []struct {
			ID      string                     `json:"_id"`
			Account data.AccountBalanceHistory `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type StatisticsAddressesBalanceEpoch struct {
	Epoch                  uint32 `json:"epoch"`
	TotalAddresses         int    `json:"totalAddresses"`
	TotalContractAddresses int    `json:"totalContractAddresses"`
	NonZero                int    `json:"nonZero"`
	B01EGLD                int    `json:"b01EGLD"`
	B1EGLD                 int    `json:"b1EGLD"`
	B10EGLD                int    `json:"b10EGLD"`
	B100EGLD               int    `json:"b100EGLD"`
	B1kEGLD                int    `json:"b1KEGLD"`
}

type SearchResponse struct {
	Hits struct {
		Hits []struct {
			ID  string          `json:"_id"`
			OBJ json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type StakeInfoEpoch struct {
	Epoch                uint32 `json:"epoch"`
	TotalStaked          string `json:"totalStaked"`
	LegacyDelegationUser int    `json:"legacyDelegationUsers"`
	LegacyDelegation     string `json:"-"`
	StakingUsers         int    `json:"stakingUsers"`
	Staking              string `json:"staking"`
	TotalUniqueUsers     int    `json:"totalUniqueUsers"`
}

type ScrollTransactionsSCRS struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []struct {
			ID string     `json:"_id"`
			Tx TxWithSCRS `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type TxWithSCRS struct {
	Nonce    uint64          `json:"nonce"`
	Receiver string          `json:"receiver"`
	Sender   string          `json:"sender"`
	Data     []byte          `json:"data"`
	Value    string          `json:"value"`
	Status   string          `json:"status"`
	SCRS     []data.ScResult `json:"scResults"`
}

// VmValuesResponseData follows the format of the data field in an API response for a VM values query
type VmValuesResponseData struct {
	Data *vm.VMOutputApi `json:"data"`
}

// ResponseVmValue defines a wrapper over string containing returned data in hex format
type ResponseVmValue struct {
	Data  VmValuesResponseData `json:"data"`
	Error string               `json:"error"`
	Code  string               `json:"code"`
}

// VmValueRequest defines the request struct for values available in a VM
type VmValueRequest struct {
	Address    string   `json:"scAddress"`
	FuncName   string   `json:"funcName"`
	CallerAddr string   `json:"caller"`
	CallValue  string   `json:"value"`
	Args       []string `json:"args"`
}

// GenericAPIResponse defines the structure of all responses on API endpoints
type GenericAPIResponse struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
	Code  string          `json:"code"`
}
