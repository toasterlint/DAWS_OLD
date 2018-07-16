package dao

import (
	"time"

	commonModels "github.com/toasterlint/DAWS/common/models"
	. "github.com/toasterlint/DAWS/common/utils"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// DAO Data Access Object
type DAO struct {
	Server   string
	Database string
	Username string
	Password string
}

var db *mgo.Database

const (
	// COLLECTIONPEOPLE People collection to use in DB
	COLLECTIONPEOPLE = "people"
	// COLLECTIONCITY City collection to use in DB
	COLLECTIONCITY = "city"
	// COLLECTIONBUILDING Building collection to use in DB
	COLLECTIONBUILDING = "building"
)

// Connect to DB
func (m *DAO) Connect() {
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

// CreateCity Creates a city in DB
func (m *DAO) CreateCity(city commonModels.City) error {
	err := db.C(COLLECTIONCITY).Insert(&city)
	return err
}

func (m *DAO) UpdateCity(city commonModels.City) error {
	err := db.C(COLLECTIONCITY).UpdateId(city.ID, &city)
	return err
}

// GetCitiesCount get number of cities in the world
func (m *DAO) GetCitiesCount() (int, error) {
	citiesCount, err := db.C(COLLECTIONCITY).Find(bson.M{}).Count()
	return citiesCount, err
}

// CreateBuilding Creates a city in DB
func (m *DAO) CreateBuilding(building commonModels.Building) error {
	err := db.C(COLLECTIONBUILDING).Insert(&building)
	return err
}

// UpdateBuilding updates a building
func (m *DAO) UpdateBuilding(building commonModels.Building) error {
	err := db.C(COLLECTIONBUILDING).UpdateId(building.ID, &building)
	return err
}

// GetBuildingsCount get number of buildings in the world
func (m *DAO) GetBuildingsCount() (int, error) {
	buildingsCount, err := db.C(COLLECTIONBUILDING).Find(bson.M{}).Count()
	return buildingsCount, err
}

// CreatePerson Creates a city in DB
func (m *DAO) CreatePerson(person commonModels.Person) error {
	err := db.C(COLLECTIONPEOPLE).Insert(&person)
	return err
}

// GetPeopleCount get number of people in the world
func (m *DAO) GetPeopleCount() (int, error) {
	peopleCount, err := db.C(COLLECTIONPEOPLE).Find(bson.M{}).Count()
	return peopleCount, err
}
