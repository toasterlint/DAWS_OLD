package models

import (
	"gopkg.in/mgo.v2/bson"
)

// Disease types of diseases
type Disease struct {
	Name            string  `json:"name" bson:"name"`
	DaysDetected    int     `json:"daysDetected" bson:"daysDetected"`
	AvgDaysIll      int     `json:"avgDaysIll" bson:"avgDaysIll"`
	LethalityRate   float32 `json:"lethalityRate" bson:"lethalityRate"`
	Infectious      bool    `json:"infectious" bson:"infectious"`
	InfectionChance float32 `json:"infectionChance" bson:"infectionChance"`
	Severity        float32 `json:"severity" bson:"severity"`
}

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
	ID                      bson.ObjectId `json:"id" bson:"_id,omitempty"`
	ViolentCrimeRate        float32       `json:"violentCrimeRate" bson:"violentCrimeRate"`
	MurderRate              float32       `json:"murderRate" bson:"murderRate"`
	CarAccidentFatalityRate float32       `json:"carAccidentFatalityRate" bson:"carAccidentFatalityRate"`
	Diseases                []Disease     `json:"diseases" bson:"diseases"`
	WorldSpeed              int           `json:"worldSpeed" bson:"worldSpeed"`
	LastTime                string        `json:"lastTime" bson:"lastTime"`
	Triggers                []Trigger     `json:"triggers" bson:"triggers"`
	SpeedLimits             []SpeedLimit  `json:"speedLimits" bson:"speedLimits"`
}

type WorldQueueMessage struct {
	Controller string `json:"controller"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
}

type WorldTrafficQueueMessage struct {
	WorldSettings Settings `json:"worldSettings"`
	Datetime      string   `json:"datetime"`
}

type WorldCityQueueMessage struct {
	City     string `json:"city"`
	Datetime string `json:"datetime"`
}

type Controller struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Ready bool   `json:"ready"`
}
