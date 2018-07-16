package models

import (
	commonModels "github.com/toasterlint/DAWS/common/models"
	"gopkg.in/mgo.v2/bson"
)

// Trigger triggers for the world
type Trigger struct {
	Name  string `json:"name" bson:"name"`
	Value string `json:"value" bson:"value"`
}

// SpeedLimit
type SpeedLimit struct {
	Location string `json:"location" bson:"location"`
	Value    int    `json:"value" bson:"value"`
}

// Settings settings for the world
type Settings struct {
	ID                      bson.ObjectId          `json:"id" bson:"_id,omitempty"`
	ViolentCrimeRate        float32                `json:"violentCrimeRate" bson:"violentCrimeRate"`
	MurderRate              float32                `json:"murderRate" bson:"murderRate"`
	CarAccidentFatalityRate float32                `json:"carAccidentFatalityRate" bson:"carAccidentFatalityRate"`
	Diseases                []commonModels.Disease `json:"diseases" bson:"diseases"`
	WorldSpeed              int                    `json:"worldSpeed" bson:"worldSpeed"`
	LastTime                string                 `json:"lastTime" bson:"lastTime"`
	Triggers                []Trigger              `json:"triggers" bson:"triggers"`
	SpeedLimits             []SpeedLimit           `json:"speedLimits" bson:"speedLimits"`
}

// WorldQueueMessage Messages sent to World Queue
type WorldQueueMessage struct {
	Controller string `json:"controller"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
}

// WorldTrafficQueueMessage Messages sent to World Traffic Queue
type WorldTrafficQueueMessage struct {
	WorldSettings Settings `json:"worldSettings"`
	Datetime      string   `json:"datetime"`
}

// WorldCityQueueMessage Messages sent to World City Queue
type WorldCityQueueMessage struct {
	WorldSettings Settings `json:"worldSettings"`
	City          string   `json:"city"`
	Datetime      string   `json:"datetime"`
}

type ControllerType int

const (
	Traffic ControllerType = iota + 1
	City
)

// Controller a controller
type Controller struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Ready bool   `json:"ready"`
	Exit  bool   `json:"exit"`
}
