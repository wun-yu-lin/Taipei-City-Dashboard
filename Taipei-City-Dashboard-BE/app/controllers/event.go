package controllers

import (
	"TaipeiCityDashboardBE/app/elk"
	"TaipeiCityDashboardBE/logs"
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type EventType string

var (
	ComponentEvent EventType = "component_event"
)

type ComponentEventPayLoad struct {
	ComponentIndex string `json:"component_index"`
	ComponentName  string `json:"component_name"`
	Event          struct {
		EnterTime time.Time `json:"enter_time"`
		LeaveTime time.Time `json:"leave_time"`
		DeviceID  string    `json:"device_id"`
	} `json:"event"`
}

type ComponentEventMesssage struct {
	EventType      EventType `json:"event_type"`
	ComponentIndex string    `json:"component_index"`
	ComponentName  string    `json:"component_name"`
	DurationSec    int       `json:"duration_sec"`
	EnterTime      time.Time `json:"enter_time"`
	LeaveTime      time.Time `json:"leave_time"`
	DeviceID       string    `json:"device_id"`
}

func PushEventMessageToELK(c *gin.Context) {

	var payload ComponentEventPayLoad
	if err := c.ShouldBindJSON(&payload); err != nil {
		logs.Error(c.Request.RequestURI+" bind json error:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var durationSec int
	if !payload.Event.EnterTime.IsZero() && !payload.Event.LeaveTime.IsZero() {
		duration := payload.Event.LeaveTime.Sub(payload.Event.EnterTime)
		durationSec = int(duration.Seconds())
	}

	//prepare message
	message := ComponentEventMesssage{
		EventType:      ComponentEvent,
		ComponentIndex: payload.ComponentIndex,
		ComponentName:  payload.ComponentName,
		EnterTime:      payload.Event.EnterTime,
		LeaveTime:      payload.Event.LeaveTime,
		DeviceID:       payload.Event.DeviceID,
		DurationSec:    durationSec,
	}

	if err := elk.MessageWorker.Submit(message); err != nil {
		logs.Error("PushEventMessageToELK elk submit error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to push to ELK"})
		return
	}
	//http 204
	c.Status(http.StatusNoContent)
	return
}

type ComponentDurationResponse struct {
	TotalCount          int     `json:"total_count"`
	AverageDurationSec  float64 `json:"average_duration_sec"`
	MeasuredOverMinutes int     `json:"measured_over_minutes"`
	MeasuredStart       string  `json:"measured_start"`
	MeasuredEnd         string  `json:"measured_end"`
}

func GetEventInfoByComponentId(c *gin.Context) {
	componentID := c.Param("id")
	minutesStr := c.DefaultQuery("minutes", "60")
	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid minutes"})
		return
	}
	endTime := time.Now().UTC()
	startTime := endTime.Add(-30 * time.Minute)

	payload := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"component_index": componentID,
						},
					},
					map[string]interface{}{
						"range": map[string]interface{}{
							"enter_time": map[string]interface{}{
								"gte": startTime.Format(time.RFC3339Nano),
								"lte": endTime.Format(time.RFC3339Nano),
							},
						},
					},
				},
			},
		},
		"aggs": map[string]interface{}{
			"avg_duration_sec": map[string]interface{}{
				"avg": map[string]interface{}{
					"field": "duration_sec",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logs.Error("GetEventInfoByComponentId marshal payload error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// 使用 elk.ESClient 直接執行搜尋
	res, err := elk.ESClient.Search(
		elk.ESClient.Search.WithContext(context.Background()),
		elk.ESClient.Search.WithIndex("logstash-*"),
		elk.ESClient.Search.WithBody(bytes.NewReader(body)),
		elk.ESClient.Search.WithSize(0),
	)
	if err != nil {
		logs.Error("Elasticsearch search error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Elasticsearch query failed"})
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		logs.Error("Elasticsearch response error: " + res.String())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Elasticsearch returned error"})
		return
	}

	// 解析 ES 回傳 JSON
	var rawResp struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations struct {
			AvgDurationSec struct {
				Value float64 `json:"value"`
			} `json:"avg_duration_sec"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&rawResp); err != nil {
		logs.Error("GetEventInfoByComponentId decode response error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse Elasticsearch response"})
		return
	}
	logs.Info(rawResp)
	response := &ComponentDurationResponse{
		TotalCount:          rawResp.Hits.Total.Value,
		AverageDurationSec:  rawResp.Aggregations.AvgDurationSec.Value,
		MeasuredOverMinutes: minutes,
		MeasuredStart:       startTime.Format(time.RFC3339Nano),
		MeasuredEnd:         endTime.Format(time.RFC3339Nano),
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": response})
}
