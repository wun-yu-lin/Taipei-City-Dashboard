package controllers

import (
	"TaipeiCityDashboardBE/app/elk"
	"TaipeiCityDashboardBE/logs"
	"github.com/gin-gonic/gin"
	"net/http"
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

func GetEventInfoByComponentId(c *gin.Context) {

}
