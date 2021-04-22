package elasticClient

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/tidwall/gjson"
)

type elasticClient struct {
	client *elasticsearch.Client
}

func NewElasticClient(cfg elasticsearch.Config) (*elasticClient, error) {
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot create database reader %w", err)
	}

	return &elasticClient{
		client: client,
	}, nil
}

func (ec *elasticClient) DoScrollRequestAllDocuments(
	query *bytes.Buffer,
	index string,
	handlerFunc func(responseBytes []byte) error,
) error {

	// use a random interval in order to avoid AWS GET request cashing
	randomNum := rand.Intn(50000)
	res, err := ec.client.Search(
		ec.client.Search.WithSize(9000),
		ec.client.Search.WithScroll(10*time.Minute+time.Duration(randomNum)*time.Millisecond),
		ec.client.Search.WithContext(context.Background()),
		ec.client.Search.WithIndex(index),
		ec.client.Search.WithBody(query),
	)
	if err != nil {
		return err
	}

	bodyBytes, err := getBytesFromResponse(res)
	if err != nil {
		return err
	}

	err = handlerFunc(bodyBytes)
	if err != nil {
		return err
	}

	scrollID := gjson.Get(string(bodyBytes), "_scroll_id")
	return ec.iterateScroll(scrollID.String(), handlerFunc)
}

func (ec *elasticClient) iterateScroll(
	scrollID string,
	handlerFunc func(responseBytes []byte) error,
) error {
	if scrollID == "" {
		return nil
	}
	defer func() {
		err := ec.clearScroll(scrollID)
		if err != nil {
			log.Print("cannot clear scroll ", err)
		}
	}()

	for {
		scrollBodyBytes, errScroll := ec.getScrollResponse(scrollID)
		if errScroll != nil {
			return errScroll
		}

		numberOfHits := gjson.Get(string(scrollBodyBytes), "hits.hits.#")
		if numberOfHits.Int() < 1 {
			return nil
		}
		err := handlerFunc(scrollBodyBytes)
		if err != nil {
			return err
		}
	}

}

func (ec *elasticClient) getScrollResponse(scrollID string) ([]byte, error) {
	randomNum := rand.Intn(10000)
	res, err := ec.client.Scroll(
		ec.client.Scroll.WithScrollID(scrollID),
		ec.client.Scroll.WithScroll(2*time.Minute+time.Duration(randomNum)*time.Millisecond),
	)
	if err != nil {
		return nil, err
	}

	return getBytesFromResponse(res)
}

func getBytesFromResponse(res *esapi.Response) ([]byte, error) {
	if res.IsError() {
		return nil, fmt.Errorf("error response: %s", res)
	}
	defer closeBody(res)

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return bodyBytes, nil
}

func (ec *elasticClient) clearScroll(scrollID string) error {
	resp, err := ec.client.ClearScroll(
		ec.client.ClearScroll.WithScrollID(scrollID),
	)
	if err != nil {
		return err
	}
	if resp.IsError() && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("error response: %s", resp)
	}

	defer closeBody(resp)

	return nil
}

func closeBody(res *esapi.Response) {
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
}

// DoSearchRequest wil do a search request to elaticsearch server
func (ec *elasticClient) DoSearchRequest(query *bytes.Buffer, index string) ([]byte, error) {
	randomNum := rand.Intn(10000)
	timeout := 5*time.Minute + time.Duration(randomNum)*time.Millisecond
	res, err := ec.client.Search(
		ec.client.Search.WithBody(query),
		ec.client.Search.WithIndex(index),
		ec.client.Search.WithTimeout(timeout),
	)
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		return nil, fmt.Errorf("%s", res.String())
	}

	defer closeBody(res)

	bodyBytes, errRead := ioutil.ReadAll(res.Body)
	if errRead != nil {
		return nil, errRead
	}

	return bodyBytes, nil
}
