package elk

import (
	"TaipeiCityDashboardBE/global"
	"TaipeiCityDashboardBE/logs"
	"bytes"
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"strings"
)

var ESClient *elasticsearch.Client

func InitESClient() {
	cfg := elasticsearch.Config{
		Addresses: []string{global.ELK.ElasticSearchURL},
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logs.Error("failed to create ES client:", err)
		return
	}

	res, err := client.Info()
	if err != nil {
		if !strings.Contains(err.Error(), "not a supported distribution of Elasticsearch") {
			logs.Error("ES info error:", err)
			return
		}
		logs.Info("ES info warning (non-official distribution), continuing…")
	}
	if res != nil {
		defer res.Body.Close()
		if res.IsError() {
			logs.Error("ES info returned error status:", res.String())
		}
	}

	ESClient = client
	logs.Info("Elasticsearch client initialized (OSS 7.x compatible)")
}

// Search performs a generic search on the given indices using the provided query
func Search[T any](indices []string, queryBody interface{}, size int) ([]T, error) {
	if ESClient == nil {
		logs.Error("Elasticsearch client is not initialized")
		return nil, nil
	}

	payload := map[string]interface{}{"query": queryBody}
	raw, err := json.Marshal(payload)
	if err != nil {
		logs.Error("failed to marshal query body:", err)
		return nil, err
	}

	req := esapi.SearchRequest{
		Index: indices,
		Body:  bytes.NewReader(raw),
		Size:  &size,
	}

	res, err := req.Do(context.Background(), ESClient)
	if err != nil {
		logs.Error("ES search error:", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		logs.Error("ES search returned error:", res.String())
		return nil, err
	}

	var resp struct {
		Hits struct {
			Hits []struct {
				Source T `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		logs.Error("failed to decode ES response:", err)
		return nil, err
	}

	results := make([]T, len(resp.Hits.Hits))
	for i, hit := range resp.Hits.Hits {
		results[i] = hit.Source
	}
	return results, nil
}
