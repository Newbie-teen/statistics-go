package process

import (
	"testing"

	"github.com/ElrondNetwork/statistics-go/elasticClient"
	"github.com/elastic/go-elasticsearch/v7"
)

func TestGetTxs(t *testing.T) {
	elsaticC, _ := elasticClient.NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})

	tp, _ := NewTransactionsProcessor(elsaticC, "../genesis")
	tp.ProcessAllTxs()
}
