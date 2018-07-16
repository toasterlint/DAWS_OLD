package dao

import (
	"time"

	commonModels "github.com/toasterlint/DAWS/common/models"
	. "github.com/toasterlint/DAWS/common/utils"
	worldModels "github.com/toasterlint/DAWS/world_controller/models"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// WorldDAO Data Access Object
type WorldDAO struct {
	Server   string
	Database string
	Username string
	Password string
}

var db *mgo.Database

const (
	// COLLECTIONSETTINGS Settings collection to use in DB
	COLLECTIONSETTINGS = "settings"
	// COLLECTIONPEOPLE People collection to use in DB
	COLLECTIONPEOPLE = "people"
	// COLLECTIONCITY City collection to use in DB
	COLLECTIONCITY = "city"
	// COLLECTIONBUILDING Building collection to use in DB
	COLLECTIONBUILDING = "building"
)

// Connect to DB
func (m *WorldDAO) Connect() {
	info := &mgo.DialInfo{
		Addrs:    []string{m.Server},
		Timeout:  60 * time.Second,
		Database: m.Database,
		Username: m.Username,
		Password: m.Password,
	}
	session, err := mgo.DialWithInfo(info)
	FailOnError(err, "Failed to connect to MongoDB")
	db = session.DB(m.Database)
}

// SaveSettings save settings to DB
func (m *WorldDAO) SaveSettings(settings worldModels.Settings) error {
	err := db.C(COLLECTIONSETTINGS).UpdateId(settings.ID, &settings)
	return err
}

// LoadSettings load settings from DB
func (m *WorldDAO) LoadSettings() (worldModels.Settings, error) {
	var settings []worldModels.Settings
	err := db.C(COLLECTIONSETTINGS).Find(bson.M{}).All(&settings)
	if len(settings) > 0 {
		return settings[0], err
	}
	return worldModels.Settings{}, err
}

// InsertSettings create settings in DB
func (m *WorldDAO) InsertSettings(settings worldModels.Settings) error {
	err := db.C(COLLECTIONSETTINGS).Insert(&settings)
	return err
}

// CreateCity Creates a city in DB
func (m *WorldDAO) CreateCity(city commonModels.City) error {
	err := db.C(COLLECTIONCITY).Insert(&city)
	return err
}

func (m *WorldDAO) UpdateCity(city commonModels.City) error {
	err := db.C(COLLECTIONCITY).UpdateId(city.ID, &city)
	return err
}

// GetCitiesCount get number of cities in the world
func (m *WorldDAO) GetCitiesCount() (int, error) {
	citiesCount, err := db.C(COLLECTIONCITY).Find(bson.M{}).Count()
	return citiesCount, err
}

// CreateBuilding Creates a city in DB
func (m *WorldDAO) CreateBuilding(building commonModels.Building) error {
	err := db.C(COLLECTIONBUILDING).Insert(&building)
	return err
}

func (m *WorldDAO) UpdateBuilding(building commonModels.Building) error {
	err := db.C(COLLECTIONBUILDING).UpdateId(building.ID, &building)
	return err
}

// GetBuildingsCount get number of buildings in the world
func (m *WorldDAO) GetBuildingsCount() (int, error) {
	buildingsCount, err := db.C(COLLECTIONBUILDING).Find(bson.M{}).Count()
	return buildingsCount, err
}

// CreatePerson Creates a city in DB
func (m *WorldDAO) CreatePerson(person commonModels.Person) error {
	err := db.C(COLLECTIONPEOPLE).Insert(&person)
	return err
}

// GetPeopleCount get number of people in the world
func (m *WorldDAO) GetPeopleCount() (int, error) {
	peopleCount, err := db.C(COLLECTIONPEOPLE).Find(bson.M{}).Count()
	return peopleCount, err
}
