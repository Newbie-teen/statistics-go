package process

import (
	"bytes"
)

type ElasticHandler interface {
	DoSearchRequest(query *bytes.Buffer, index string) ([]byte, error)
	DoScrollRequestAllDocuments(query *bytes.Buffer, index string, handlerFunc func(responseBytes []byte) error) error
}

// // RestClientHandler defines what a rest client should be able do
type RestClientHandler interface {
	CallGetRestEndPoint(path string, value interface{}) error
	CallPostRestEndPoint(path string, data interface{}, response interface{}) error
}

type AccountsHandler interface {
	ProcessAllAccounts(endEpoch uint32) ([]byte, error)
}

type TransactionsHandler interface {
	ProcessAllTxs(endEpoch uint32) ([]byte, error)
}

type StakeInfoHandler interface {
	ProcessEpochs(endEpoch uint32) ([]byte, error)
}
