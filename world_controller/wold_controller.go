package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/common/utils"
	. "github.com/toasterlint/DAWS/world_controller/dao"
	. "github.com/toasterlint/DAWS/world_controller/models"
	"gopkg.in/mgo.v2/bson"
)

var conn *amqp.Connection
var ch *amqp.Channel
var worldq, worldtrafficq, worldcityq amqp.Queue
var msgs <-chan amqp.Delivery
var maxTriggerTime int // smaller number equals faster speed
var runTrigger bool
var controllers []Controller
var lastTime time.Time
var settings Settings
var dao = WorldDAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}

func startHTTPServer() {
	r := mux.NewRouter()
	r.Handle("/", http.FileServer(http.Dir("./html")))
	r.HandleFunc("/api/status", apiStatus).Methods("GET")
	r.HandleFunc("/api/triggerNext", apiTrigger).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func connectQueues() {
	var err error
	conn, err = amqp.Dial("amqp://guest:guest@rabbitmq.daws.xyz:5672")
	FailOnError(err, "Failed to connect to RabbitMQ")

	ch, err = conn.Channel()
	FailOnError(err, "Failed to open a channel")

	worldq, err = ch.QueueDeclare(
		"world_queue", //name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	FailOnError(err, "Failed to declare queue")

	worldtrafficq, err = ch.QueueDeclare(
		"world_traffic_queue", //name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	FailOnError(err, "Failed to declare queue")

	Logger.Printf("World Traffic Queue Consumers: %d", worldtrafficq.Consumers)

	worldcityq, err = ch.QueueDeclare(
		"world_city_queue", //name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	FailOnError(err, "Failed to declare queue")

	Logger.Printf("World City Queue Consumers: %d", worldcityq.Consumers)

	err = ch.Qos(
		1,     //prefetch count
		0,     //prefetch size
		false, //global
	)

	msgs, err = ch.Consume(
		worldq.Name, //queue
		"",          //consumer
		false,       //auto-ack
		false,       //exclusive
		false,       //no-local
		false,       //no-wait
		nil,         //args
	)
	FailOnError(err, "Failed to register a consumer")
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	LogToConsole("API Call made: status")
	w.Write([]byte("API Call made: status"))
}

func apiTrigger(w http.ResponseWriter, r *http.Request) {
	cities := []string{"Orlando", "Green Bay", "Chicago", "Seattle"}
	msg := &WorldTrafficQueueMessage{WorldSettings: settings, Datetime: lastTime.Format("2006-01-02 15:04:05")}
	triggerNext(cities, msg)
	LogToConsole("Manually Trigger")
	w.Write([]byte("Manually triggered"))
}

func triggerNext(cities []string, worldtrafficmessage *WorldTrafficQueueMessage) {
	tempMsgJSON, _ := json.Marshal(worldtrafficmessage)
	err := ch.Publish(
		"",                 // exchange
		worldtrafficq.Name, // routing key
		false,              // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         []byte(tempMsgJSON),
		})
	FailOnError(err, "Failed to post to World Traffic Queue")
	for _, element := range cities {
		tempMsg := &WorldCityQueueMessage{WorldSettings: settings, City: element, Datetime: worldtrafficmessage.Datetime}
		tempMsgJSON, _ := json.Marshal(tempMsg)
		err := ch.Publish(
			"",              // exchange
			worldcityq.Name, // routing key
			false,           // mandatory
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         []byte(tempMsgJSON),
			})
		FailOnError(err, "Failed to post to World Traffic Queue")
	}
}

func processTrigger() {
	realLastTime := time.Now()
	for runTrigger {
		// first check if all controllers are ready (and that we have any)
		if len(controllers) == 0 {
			LogToConsole("No controllers")
			time.Sleep(time.Second * 5)
			continue
		}
		readyt := true
		readyc := true
		totalt := 0
		totalc := 0
		for i := range controllers {
			if controllers[i].Type == "traffic" {
				totalt++
			} else {
				totalc++
			}
			if controllers[i].Ready == false {
				if controllers[i].Type == "traffic" {
					readyt = false
				} else {
					readyc = false
				}
				break
			}
		}
		if readyt == false || readyc == false {
			continue
		}
		if totalt == 0 || totalc == 0 {
			continue
		}
		// make sure we don't go over max speed limit
		t := time.Now()
		dur := t.Sub(realLastTime)
		LogToConsole("Trigger Ding!")
		for i := range controllers {
			controllers[i].Ready = false
		}
		if dur > time.Duration(maxTriggerTime)*time.Millisecond {
			LogToConsole("Warning: world processing too slow, last duration was - " + dur.String())
		}
		cities := []string{"Orlando", "Green Bay", "Chicago", "Seattle"}
		msg := &WorldTrafficQueueMessage{WorldSettings: settings, Datetime: lastTime.Format("2006-01-02 15:04:05")}
		triggerNext(cities, msg)
		lastTime = lastTime.Add(time.Second * 1)
		realLastTime = time.Now()
	}
}

func processMsgs() {
	for d := range msgs {
		bodyString := string(d.Body[:])
		LogToConsole("Received a message: " + bodyString)
		tempController := Controller{}
		json.Unmarshal(d.Body, &tempController)
		found := false
		var tempRemove int
		for i := range controllers {
			if controllers[i].ID == tempController.ID {
				found = true
				controllers[i].Ready = tempController.Ready
				tempRemove = i
				break
			}
		}
		//LogToConsole("Controller found stats: " + strconv.FormatBool(found))
		if found == false {
			controllers = append(controllers, tempController)
		}
		// Remove the controller if it sent exit true
		if tempController.Exit == true {
			controllers = append(controllers[:tempRemove], controllers[tempRemove+1:]...)
		}
		LogToConsole("Done")
		d.Ack(false)
	}
}

func runConsole() {
	// setup terminal
	reader := bufio.NewReader(os.Stdin)
ReadCommand:
	Logger.Print("Command: ")
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\n")
	switch text {
	case "exit":
		Logger.Print("Purging queues")
		_, err := ch.QueuePurge(worldcityq.Name, false)
		FailOnError(err, "Failed to purge World City Queue")
		_, err = ch.QueuePurge(worldtrafficq.Name, false)
		FailOnError(err, "Failed to purge World Traffic Queue")
		_, err = ch.QueuePurge(worldq.Name, false)
		FailOnError(err, "Failed to purge World Queue")
		Logger.Println("Saving settings...")
		settings.LastTime = lastTime.Format("2006-01-02 15:04:05")
		err = dao.SaveSettings(settings)
		FailOnError(err, "Failed to save settings")
		Logger.Println("Exiting...")
		os.Exit(0)
	case "status":
		if runTrigger {
			Logger.Println("Running...")
		} else {
			Logger.Println("Stopped...")
		}

		tcontrollers := 0
		ccontrollers := 0
		for i := range controllers {
			if controllers[i].Type == "traffic" {
				tcontrollers++
			} else {
				ccontrollers++
			}
		}
		Logger.Printf("Traffic Controllers: %d", tcontrollers)
		Logger.Printf("City Controllers: %d", ccontrollers)
		Logger.Printf("Current Real Time: %s", time.Now().Format("2006-01-02 15:04:05"))
		Logger.Printf("Current Simulated Time: %s", lastTime.Format("2006-01-02 15:04:05"))
	case "help":
		fallthrough
	default:
		Logger.Println("Help: ")
		Logger.Println("   status - Check the status of the world")
		Logger.Println("   exit - Exit the App")
	}
	goto ReadCommand
}

func loadConfig() {
	dao.Connect()
	var err error
	settings, err = dao.LoadSettings()
	FailOnError(err, "Failed to load settings")
	if settings.ID.Valid() {
		sett, _ := json.Marshal(settings)
		LogToConsole(string(sett))
	} else {
		LogToConsole("No settings found, creating defaults")
		var tempSettings Settings
		tempSettings.CarAccidentFatalityRate = 0.0001159
		tempSettings.ID = bson.NewObjectId()
		tempSettings.LastTime = time.Now().Format("2006-01-02 15:04:05")
		tempSettings.MurderRate = 0.000053
		tempSettings.ViolentCrimeRate = 0.00381
		tempSettings.WorldSpeed = 5000
		var speeds = []SpeedLimit{}
		var citySpeed = SpeedLimit{Location: "city", Value: 35}
		var noncitySpeed = SpeedLimit{Location: "noncity", Value: 70}
		speeds = append(speeds, citySpeed)
		speeds = append(speeds, noncitySpeed)
		tempSettings.SpeedLimits = speeds
		tempSettings.Diseases = []Disease{}
		err := dao.InsertSettings(tempSettings)
		FailOnError(err, "Failed to insert settings")
	}
	// Need to use lastTime since settings.LastTime is a string and we need to do time math
	timeLayout := "2006-01-02 15:04:05"
	lastTime, err = time.Parse(timeLayout, settings.LastTime)
}

func main() {
	// Set some initial variables
	InitLogger()
	loadConfig()
	runTrigger = true
	controllers = []Controller{}

	//init rabbit
	connectQueues()
	defer conn.Close()
	defer ch.Close()
	go processMsgs()

	// Start Web Server
	go startHTTPServer()

	// Start Console
	go runConsole()

	// TEMP: Start trigger
	go processTrigger()

	// Loop main thread
	forever := make(chan bool)
	<-forever

	Logger.Println("done")
}

func getCities() {

}

func getPeople() {

}
