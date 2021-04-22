package process

import (
	"testing"

	"github.com/ElrondNetwork/elrond-go/config"
	"github.com/ElrondNetwork/elrond-go/data/state/factory"
	"github.com/ElrondNetwork/statistics-go/elasticClient"
	"github.com/ElrondNetwork/statistics-go/restClient"
	"github.com/elastic/go-elasticsearch/v7"
)

func TestStakeInfoProcessor_ProcessEpochs(t *testing.T) {
	elsaticC, _ := elasticClient.NewElasticClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	restClientt, _ := restClient.NewRestClient("https://gateway.elrond.com")
	pubKeyConverter, _ := factory.NewPubkeyConverter(config.PubkeyConfig{Type: "bech32", Length: 32})

	ap, _ := NewStakeInfoProcessor(elsaticC, restClientt, pubKeyConverter, "../genesis", 1596117600)

	ap.ProcessEpochs(50)
	//ap.getAllDelegationManagerContracts()
}
