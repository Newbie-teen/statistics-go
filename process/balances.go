package process

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path"

	"github.com/ElrondNetwork/statistics-go/data"
)

func DumpBalances(
	delegationLegacyUsers map[string]*big.Int,
	stakingUsers map[string]*big.Int,
	pathToFolder string,
	epoch uint32,
) {
	checkBalances2(delegationLegacyUsers, "delegation legacy")
	checkBalances2(stakingUsers, "staking")

	mapUniqueUsers := map[string]string{}
	for key, value := range stakingUsers {
		mapUniqueUsers[key] = value.String()
	}

	for key, value := range delegationLegacyUsers {
		_, ok := mapUniqueUsers[key]
		if !ok {
			mapUniqueUsers[key] = value.String()
			continue
		}

		mapUniqueUsers[key] = big.NewInt(0).Add(stringToBigInt(mapUniqueUsers[key]), value).String()
	}

	checkBalances(mapUniqueUsers)

	bytes, _ := json.MarshalIndent(mapUniqueUsers, "", " ")
	_ = ioutil.WriteFile(path.Join(pathToFolder, fmt.Sprintf("epoch%d.json", epoch)), bytes, 0644)
}

func checkBalances2(accts map[string]*big.Int, message string) {
	fmt.Println(message)
	currentEpochStats := &data.StatisticsAddressesBalanceEpoch{}
	for _, b := range accts {
		currentBalance := big.NewInt(0).SetBytes(b.Bytes())
		if currentBalance.Cmp(nonZero) > 0 {
			currentEpochStats.NonZero++
		}

		if currentBalance.Cmp(b01EGLD) >= 0 {
			currentEpochStats.B01EGLD++
		}

		if currentBalance.Cmp(b1EGLD) >= 0 {
			currentEpochStats.B1EGLD++
		}

		if currentBalance.Cmp(b10EGLD) >= 0 {
			currentEpochStats.B10EGLD++
		}

		if currentBalance.Cmp(b100EGLD) >= 0 {
			currentEpochStats.B100EGLD++
		}

		if currentBalance.Cmp(b1KEGLD) >= 0 {
			currentEpochStats.B1kEGLD++
		}
	}

	bytes, _ := json.MarshalIndent(currentEpochStats, "", " ")
	fmt.Println(string(bytes))
}

func checkBalances(accts map[string]string) {
	currentEpochStats := &data.StatisticsAddressesBalanceEpoch{}
	for _, b := range accts {
		currentBalance := big.NewInt(0).SetBytes(stringToBigInt(b).Bytes())
		if currentBalance.Cmp(nonZero) > 0 {
			currentEpochStats.NonZero++
		}

		if currentBalance.Cmp(b01EGLD) >= 0 {
			currentEpochStats.B01EGLD++
		}

		if currentBalance.Cmp(b1EGLD) >= 0 {
			currentEpochStats.B1EGLD++
		}

		if currentBalance.Cmp(b10EGLD) >= 0 {
			currentEpochStats.B10EGLD++
		}

		if currentBalance.Cmp(b100EGLD) >= 0 {
			currentEpochStats.B100EGLD++
		}

		if currentBalance.Cmp(b1KEGLD) >= 0 {
			currentEpochStats.B1kEGLD++
		}
	}

	bytes, _ := json.MarshalIndent(currentEpochStats, "", " ")
	fmt.Println(string(bytes))
}

func ReadBalances(pathToBalances string, epoch uint32) (map[string]string, error) {
	byteValue, err := getBytesFromJson(path.Join(pathToBalances, fmt.Sprintf("epoch%d.json", epoch)))
	if err != nil {
		return nil, err
	}

	balances := map[string]string{}
	err = json.Unmarshal(byteValue, &balances)
	if err != nil {
		return nil, err
	}

	return balances, nil
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
