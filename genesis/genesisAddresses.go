package genesis

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"os"
	"path"
)

// GenesisStruct -
type Genesis struct {
	Adr        string `json:"address"`
	Delegation struct {
		Value string `json:"value"`
	} `json:"delegation"`
}

type NodesSetup struct {
	InitialNodes []struct {
		Addr string `json:"address"`
	} `json:"initialNodes"`
}

func ReadGenesisAddresses(pathToFiles string) (map[string]struct{}, error) {
	byteValue, err := getBytesFromJson(path.Join(pathToFiles, "genesis.json"))
	if err != nil {
		return nil, err
	}

	genesisAccts := make([]Genesis, 0)
	err = json.Unmarshal(byteValue, &genesisAccts)
	if err != nil {
		return nil, err
	}

	genesisAccounts := make(map[string]struct{})
	for _, acct := range genesisAccts {
		genesisAccounts[acct.Adr] = struct{}{}
	}

	byteValue, err = getBytesFromJson(path.Join(pathToFiles, "nodesSetup.json"))
	if err != nil {
		return nil, err
	}

	nodes := NodesSetup{}
	err = json.Unmarshal(byteValue, &nodes)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.InitialNodes {
		genesisAccounts[node.Addr] = struct{}{}
	}

	genesisAccounts["erd1qqqqqqqqqqqqqqqpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqplllst77y4l"] = struct{}{}
	genesisAccounts["erd1qqqqqqqqqqqqqpgqxwakt2g7u9atsnr03gqcgmhcv38pt7mkd94q6shuwt"] = struct{}{}

	return genesisAccounts, nil
}

func ReadGenesisDelegationLegacyUsers(pathToFiles string) (map[string]*big.Int, error) {
	byteValue, err := getBytesFromJson(path.Join(pathToFiles, "genesis.json"))
	if err != nil {
		return nil, err
	}

	genesisAccts := make([]Genesis, 0)
	err = json.Unmarshal(byteValue, &genesisAccts)
	if err != nil {
		return nil, err
	}

	genesisAccounts := make(map[string]*big.Int)
	for _, acct := range genesisAccts {
		if acct.Delegation.Value == "0" {
			continue
		}

		genesisAccounts[acct.Adr] = stringToBigInt(acct.Delegation.Value)
	}

	return genesisAccounts, nil
}

func ReadGenesisStakingUsers(pathToFiles string) (map[string]*big.Int, error) {
	byteValue, err := getBytesFromJson(path.Join(pathToFiles, "nodesSetup.json"))
	if err != nil {
		return nil, err
	}

	nodes := NodesSetup{}
	err = json.Unmarshal(byteValue, &nodes)
	if err != nil {
		return nil, err
	}

	b2500EGLD, _ := big.NewInt(0).SetString("2500000000000000000000", 10)
	genesisAccounts := make(map[string]*big.Int)
	for _, node := range nodes.InitialNodes {
		_, ok := genesisAccounts[node.Addr]
		if !ok {
			genesisAccounts[node.Addr] = big.NewInt(0).SetBytes(b2500EGLD.Bytes())
		}

		genesisAccounts[node.Addr].Add(genesisAccounts[node.Addr], big.NewInt(0).SetBytes(b2500EGLD.Bytes()))
	}

	return genesisAccounts, nil
}

func getBytesFromJson(pathToFile string) ([]byte, error) {
	jsonFile, err := os.Open(pathToFile)
	// if we os.Open returns an error then handle it
	if err != nil {
		return nil, err
	}

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	return byteValue, nil
}

func stringToBigInt(b string) *big.Int {
	bigV, ok := big.NewInt(0).SetString(b, 10)
	if !ok {
		return big.NewInt(0)
	}

	return bigV
}
