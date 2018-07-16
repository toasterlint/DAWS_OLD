package models

import (
	. "image"
	"time"

	"gopkg.in/mgo.v2/bson"
)

// City city
type City struct {
	ID          bson.ObjectId   `json:"id" bson:"_id,omitempty"`
	Name        string          `json:"name" bson:"name"`
	TopLeft     Point           `json:"topleft" bson:"topleft"`
	BottomRight Point           `json:"bottomright" bson:"bottomright"`
	BuildingIDs []bson.ObjectId `json:"buildingIDs" bson:"buildingIDs"`
	Established time.Time       `json:"established" bson:"established"`
}

// BuildingType used to identify the type of building
type BuildingType int

const (
	House BuildingType = iota + 1
	Apartment
	School
	Office
	Warehouse
	Retail
	Entertainment
	Hospital
	Police
)

// Building a building in a city
type Building struct {
	ID           bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name         string        `json:"name" bson:"name"`
	TopLeft      Point         `json:"topleft" bson:"topleft"`
	BottomRight  Point         `json:"bottomright" bson:"bottomright"`
	BuildDate    time.Time     `json:"builddate" bson:"builddate"`
	Type         BuildingType  `json:"type" bson:"type"`
	Floors       int           `json:"floors" bson:"floors"`
	MaxOccupancy int           `json:"maxoccupancy" bson:"maxoccupancy"`
}

// DeathType used to identify how person died
type DeathType int

const (
	Natural DeathType = iota + 1
	Accident
	Murder
	Illness
)

// Person a person
type Person struct {
	ID              bson.ObjectId   `json:"id" bson:"_id,omitempty"`
	Birthdate       time.Time       `json:"birthdate" bson:"birthdate"`
	FirstName       string          `json:"firstname" bson:"firstname"`
	LastName        string          `json:"lastname" bson:"lastname"`
	ChildrenIDs     []bson.ObjectId `json:"childrenIDs" bson:"childrenIDs"`
	CurrentBuilding bson.ObjectId   `json:"currentbuilding" bson:"currentbuilding,omitempty"`
	CurrentXY       Point           `json:"currentxy" bson:"currentxy"`
	Traveling       bool            `json:"traveling" bson:"traveling"`
	NewToBuilding   bool            `json:"newtobuilding" bson:"newtobuilding"`
	HomeBuilding    bson.ObjectId   `json:"homebuilding" bson:"homebuilding,omitempty"`
	WorkBuilding    bson.ObjectId   `json:"workbuilding" bson:"workbuilding,omitempty"`
	Health          int             `json:"health" bson:"health"`
	Illness         bson.ObjectId   `json:"illness" bson:"illness,omitempty"`
	Happiness       int             `json:"happiness" bson:"happiness"`
	DeathDate       time.Time       `json:"deathdate" bson:"deathdate"`
	CauseOfDeath    DeathType       `json:"causeofdeath" bson:"causeofdeath"`
	Spouse          bson.ObjectId   `json:"spouse" bson:"spouse"`
}

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
