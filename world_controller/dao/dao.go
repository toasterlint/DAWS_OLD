package dao

import (
	"fmt"
	"log"
	"time"

	. "github.com/toasterlint/DAWS/world_controller/models"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type WorldDAO struct {
	Server   string
	Database string
	Username string
	Password string
}

var db *mgo.Database

const (
	COLLECTION = "settings"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

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
	failOnError(err, "Failed to connect to MongoDB")
	db = session.DB(m.Database)
}

func (m *WorldDAO) SaveSettings(settings Settings) error {
	err := db.C(COLLECTION).UpdateId(settings.ID, &settings)
	return err
}

func (m *WorldDAO) LoadSettings() (Settings, error) {
	var settings []Settings
	err := db.C(COLLECTION).Find(bson.M{}).All(&settings)
	if len(settings) > 0 {
		return settings[0], err
	}
	return Settings{}, err
}

func (m *WorldDAO) InsertSettings(settings Settings) error {
	err := db.C(COLLECTION).Insert(&settings)
	return err
}
