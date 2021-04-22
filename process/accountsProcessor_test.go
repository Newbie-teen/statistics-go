package process

import (
	"testing"

	"github.com/ElrondNetwork/elrond-go/config"
	"github.com/ElrondNetwork/elrond-go/data/state/factory"
	"github.com/ElrondNetwork/statistics-go/elasticClient"
	"github.com/elastic/go-elasticsearch/v7"
)

func TestAp(t *testing.T) {
	pubKeyConverter, _ := factory.NewPubkeyConverter(config.PubkeyConfig{Type: "bech32", Length: 32})

	elsaticC, _ := elasticClient.NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})

	ap, _ := NewAccountsProcessor(elsaticC, pubKeyConverter, 1596117600)

	ap.ProcessAllAccounts(50)
}
