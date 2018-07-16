package dao

import (
	"time"

	commonModels "github.com/toasterlint/DAWS/common/models"
	. "github.com/toasterlint/DAWS/common/utils"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// WorldDAO Data Access Object
type DAO struct {
	Server   string
	Database string
	Username string
	Password string
}

var db *mgo.Database

const (
	// COLLECTIONSETTINGS Settings collection to use in DB
	COLLECTIONSETTINGS = "settings"
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

// SaveSettings save settings to DB
func (m *DAO) SaveSettings(settings commonModels.Settings) error {
	err := db.C(COLLECTIONSETTINGS).UpdateId(settings.ID, &settings)
	return err
}

// LoadSettings load settings from DB
func (m *DAO) LoadSettings() (commonModels.Settings, error) {
	var settings []commonModels.Settings
	err := db.C(COLLECTIONSETTINGS).Find(bson.M{}).All(&settings)
	if len(settings) > 0 {
		return settings[0], err
	}
	return commonModels.Settings{}, err
}

// InsertSettings create settings in DB
func (m *DAO) InsertSettings(settings commonModels.Settings) error {
	err := db.C(COLLECTIONSETTINGS).Insert(&settings)
	return err
}
