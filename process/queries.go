package process

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ElrondNetwork/elrond-go/core"
)

type object = map[string]interface{}

func encodeQuery(query object) (bytes.Buffer, error) {
	var buff bytes.Buffer
	if err := json.NewEncoder(&buff).Encode(query); err != nil {
		return bytes.Buffer{}, fmt.Errorf("error encoding query: %s", err.Error())
	}

	return buff, nil
}

func getTransactionsByTimestamp(start, stop int) *bytes.Buffer {
	obj := object{
		"query": object{
			"range": object{
				"timestamp": object{
					"gte": start,
					"lte": stop,
				},
			},
		},
	}

	encoded, _ := encodeQuery(obj)

	return &encoded
}

func accountsHistoryAddress(start, stop int, addr string) *bytes.Buffer {
	obj := object{
		"query": object{
			"bool": object{
				"must": []interface{}{
					object{
						"range": object{
							"timestamp": object{
								"gte": start,
								"lte": stop,
							},
						},
					},
					object{
						"match": object{
							"address": addr,
						},
					},
				},
			},
		},
		"sort": []interface{}{
			object{
				"timestamp": object{
					"order": "desc",
				},
			},
		},
		"size": 1,
	}

	encoded, _ := encodeQuery(obj)

	return &encoded
}

func rewardTxQuery(start, stop int, addr string) *bytes.Buffer {
	obj := object{
		"query": object{
			"bool": object{
				"must": []interface{}{
					object{
						"range": object{
							"timestamp": object{
								"gte": start,
								"lte": stop,
							},
						},
					},
					object{
						"match": object{
							"receiver": addr,
						},
					},
					object{
						"match": object{
							"sender": fmt.Sprintf("%d", core.MetachainShardId),
						},
					},
					object{
						"match": object{
							"status": "success",
						},
					},
				},
			},
		},
	}

	encoded, _ := encodeQuery(obj)

	return &encoded
}

func getTransactionsToAddr(start, stop int, addr string) *bytes.Buffer {
	obj := object{
		"query": object{
			"bool": object{
				"must": []interface{}{
					object{
						"range": object{
							"timestamp": object{
								"gte": start,
								"lte": stop,
							},
						},
					},
					object{
						"match": object{
							"receiver": addr,
						},
					},
				},
			},
		},
		"sort": []interface{}{
			object{
				"timestamp": object{
					"order": "asc",
				},
			},
		},
	}

	encoded, _ := encodeQuery(obj)

	return &encoded
}
