package elk

import (
	"TaipeiCityDashboardBE/global"
	"TaipeiCityDashboardBE/logs"
	"github.com/elastic/go-elasticsearch/v7"
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
