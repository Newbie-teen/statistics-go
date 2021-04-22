package process

import (
	"testing"

	"github.com/ElrondNetwork/elrond-go/config"
	"github.com/ElrondNetwork/elrond-go/data/state/factory"
	"github.com/ElrondNetwork/statistics-go/elasticClient"
	"github.com/elastic/go-elasticsearch/v7"
)

func TestGetTxs(t *testing.T) {
	elsaticC, _ := elasticClient.NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})

	pubKeyConverter, _ := factory.NewPubkeyConverter(config.PubkeyConfig{Type: "bech32", Length: 32})

	tp, _ := NewTransactionsProcessor(elsaticC, pubKeyConverter, "../genesis", 1596117600)
	tp.ProcessAllTxs(50)
}
