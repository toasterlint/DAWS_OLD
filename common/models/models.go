package models

import (
	. "image"

	"gopkg.in/mgo.v2/bson"
)

type City struct {
	ID          bson.ObjectId   `json:"id" bson:"_id,omitempty"`
	Name        string          `json:"name" bson:"name"`
	TopLeft     Point           `json:"topleft" bson:"topleft"`
	BottomRight Point           `json:"bottomright" bson:"bottomright"`
	BuildingIDs []bson.ObjectId `json:"buildingIDs" bson:"buildingIDs"`
}
