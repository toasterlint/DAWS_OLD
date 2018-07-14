package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/world_controller/dao"
	. "github.com/toasterlint/DAWS/world_controller/models"
	"gopkg.in/mgo.v2/bson"
)

type worldQueueMessage struct {
	Controller string `json:"controller"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
}

type worldTrafficQueueMessage struct {
	WorldSettings Settings `json:"worldSettings"`
	Datetime      string   `json:"datetime"`
}

type worldCityQueueMessage struct {
	City     string `json:"city"`
	Datetime string `json:"datetime"`
}

type controller struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Ready bool   `json:"ready"`
}

var conn *amqp.Connection
var ch *amqp.Channel
var worldq, worldtrafficq, worldcityq amqp.Queue
var msgs <-chan amqp.Delivery
var maxTriggerTime int // smaller number equals faster speed
var runTrigger bool
var logger *log.Logger
var controllers []controller
var lastTime time.Time
var settings Settings
var dao = WorldDAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}

func logToConsole(message string) {
	logger.Printf("\r\033[0K%s", message)
	logger.Printf("\r\033[0KCommand: ")
}

func failOnError(err error, msg string) {
	if err != nil {
		logger.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

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
	failOnError(err, "Failed to connect to RabbitMQ")

	ch, err = conn.Channel()
	failOnError(err, "Failed to open a channel")

	worldq, err = ch.QueueDeclare(
		"world_queue", //name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	failOnError(err, "Failed to declare queue")

	worldtrafficq, err = ch.QueueDeclare(
		"world_traffic_queue", //name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare queue")

	logger.Printf("World Traffic Queue Consumers: %d", worldtrafficq.Consumers)

	worldcityq, err = ch.QueueDeclare(
		"world_city_queue", //name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	failOnError(err, "Failed to declare queue")

	logger.Printf("World City Queue Consumers: %d", worldcityq.Consumers)

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
	failOnError(err, "Failed to register a consumer")
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	logToConsole("API Call made: status")
	w.Write([]byte("API Call made: status"))
}

func apiTrigger(w http.ResponseWriter, r *http.Request) {
	cities := []string{"Orlando", "Green Bay", "Chicago", "Seattle"}
	msg := &worldTrafficQueueMessage{WorldSettings: settings, Datetime: lastTime.Format("2006-01-02 15:04:05")}
	triggerNext(cities, msg)
	logToConsole("Manually Trigger")
	w.Write([]byte("Manually triggered"))
}

func triggerNext(cities []string, worldtrafficmessage *worldTrafficQueueMessage) {
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
	failOnError(err, "Failed to post to World Traffic Queue")
	for _, element := range cities {
		tempMsg := &worldCityQueueMessage{City: element, Datetime: worldtrafficmessage.Datetime}
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
		failOnError(err, "Failed to post to World Traffic Queue")
	}
}

func processTrigger() {
	realLastTime := time.Now()
	for runTrigger {
		// first check if all controllers are ready (and that we have any)
		if len(controllers) == 0 {
			logToConsole("No controllers")
			time.Sleep(time.Second * 5)
			continue
		}
		ready := true
		for i := range controllers {
			if controllers[i].Ready == false {
				ready = false
				break
			}
		}
		if ready == false {
			continue
		}
		// make sure we don't go over max speed limit
		t := time.Now()
		dur := t.Sub(realLastTime)
		logToConsole("Trigger Ding!")
		for i := range controllers {
			controllers[i].Ready = false
		}
		if dur > time.Duration(maxTriggerTime)*time.Millisecond {
			logToConsole("Warning: world processing too slow, last duration was - " + dur.String())
		}
		cities := []string{"Orlando", "Green Bay", "Chicago", "Seattle"}
		msg := &worldTrafficQueueMessage{WorldSettings: settings, Datetime: lastTime.Format("2006-01-02 15:04:05")}
		triggerNext(cities, msg)
		lastTime = lastTime.Add(time.Second * 1)
		realLastTime = time.Now()
	}
}

func processMsgs() {
	for d := range msgs {
		bodyString := string(d.Body[:])
		logToConsole("Received a message: " + bodyString)
		tempController := controller{}
		json.Unmarshal(d.Body, &tempController)
		found := false
		for i := range controllers {
			if controllers[i].ID == tempController.ID {
				found = true
				controllers[i].Ready = tempController.Ready
				break
			}
		}
		logToConsole("Controller found stats: " + strconv.FormatBool(found))
		if found == false {
			controllers = append(controllers, tempController)
		}
		logToConsole("Done")
		d.Ack(false)
	}
}

func runConsole() {
	// setup terminal
	reader := bufio.NewReader(os.Stdin)
ReadCommand:
	logger.Print("Command: ")
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\n")
	switch text {
	case "exit":
		logger.Print("Purging queues")
		_, err := ch.QueuePurge(worldcityq.Name, false)
		failOnError(err, "Failed to purge World City Queue")
		_, err = ch.QueuePurge(worldtrafficq.Name, false)
		failOnError(err, "Failed to purge World Traffic Queue")
		_, err = ch.QueuePurge(worldq.Name, false)
		failOnError(err, "Failed to purge World Queue")
		logger.Println("Saving settings...")
		err = dao.SaveSettings(settings)
		failOnError(err, "Failed to save settings")
		logger.Println("Exiting...")
		os.Exit(0)
	case "status":
		if runTrigger {
			logger.Println("Running...")
		} else {
			logger.Println("Stopped...")
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
		logger.Printf("Traffic Controllers: %d", tcontrollers)
		logger.Printf("City Controllers: %d", ccontrollers)
		logger.Printf("Current Real Time: %s", time.Now().Format("2006-01-02 15:04:05"))
		logger.Printf("Current Simulated Time: %s", lastTime.Format("2006-01-02 15:04:05"))
	case "help":
		fallthrough
	default:
		logger.Println("Help: ")
		logger.Println("   status - Check the status of the world")
		logger.Println("   exit - Exit the App")
	}
	goto ReadCommand
}

func loadConfig() {
	dao.Connect()
	var err error
	settings, err = dao.LoadSettings()
	failOnError(err, "Failed to load settings")
	if settings.ID.Valid() {
		sett, _ := json.Marshal(settings)
		logToConsole(string(sett))
	} else {
		logToConsole("No settings found, creating defaults")
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
		failOnError(err, "Failed to insert settings")
	}
}
func main() {
	// Set some initial variables
	logger = log.New(os.Stdout, "", 0)
	logger.SetPrefix("")
	loadConfig()
	runTrigger = true
	controllers = []controller{}

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

	logger.Println("done")
}
