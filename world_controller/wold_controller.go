package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

type worldQueueMessage struct {
	Controller string `json:"controller"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
}

type worldTrafficQueueMessage struct {
	Datetime string `json:"datetime"`
}

type worldCityQueueMessage struct {
	City     string `json:"city"`
	Datetime string `json:"datetime"`
}

type controller struct {
	ID    string
	Type  string
	Ready bool
}

var conn *amqp.Connection
var ch *amqp.Channel
var worldq, worldtrafficq, worldcityq amqp.Queue
var triggerTime int
var runTrigger bool
var logger *log.Logger
var controllers []controller

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
	defer conn.Close()

	ch, err = conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

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
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	logToConsole("API Call made: status")
	w.Write([]byte("API Call made: status"))
}

func apiTrigger(w http.ResponseWriter, r *http.Request) {
	cities := []string{"Orlando", "Green Bay", "Chicago", "Seattle"}
	t := time.Now()
	msg := &worldTrafficQueueMessage{t.Format("2006-01-02 15:04:05")}
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
	lastTime := time.Now()
	for runTrigger {
		// first check if all controllers are ready (and that we have any)
		if len(controllers) == 0 {
			continue
		}
		ready := true
		for _, c := range controllers {
			if c.Ready == false {
				ready = false
				continue
			}
		}
		if ready == false {
			continue
		}
		// make sure we don't go over max speed limit
		t := time.Now()
		dur := t.Sub(lastTime)
		if dur < time.Duration(triggerTime) {
			logToConsole("Trigger Ding!")
			cities := []string{"Orlando", "Green Bay", "Chicago", "Seattle"}
			t := time.Now()
			msg := &worldTrafficQueueMessage{t.Format("2006-01-02 15:04:05")}
			triggerNext(cities, msg)
		}
	}
}

func main() {
	// Set some initial variables
	triggerTime = 5000
	runTrigger = true
	logger = log.New(os.Stdout, "", 0)
	logger.SetPrefix("")
	controllers = []controller{}

	//init rabbit
	connectQueues()

	err := ch.Qos(
		1,     //prefetch count
		0,     //prefetch size
		false, //global
	)

	msgs, err := ch.Consume(
		worldq.Name, //queue
		"",          //consumer
		false,       //auto-ack
		false,       //exclusive
		false,       //no-local
		false,       //no-wait
		nil,         //args
	)
	failOnError(err, "Failed to register a consumer")

	// Start Web Server
	go startHTTPServer()

	// setup terminal
	reader := bufio.NewReader(os.Stdin)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			bodyString := string(d.Body[:])
			logToConsole("Received a message: " + bodyString)
			tempController := controller{}
			json.Unmarshal(d.Body, &tempController)
			found := false
			for _, c := range controllers {
				if tempController.ID == c.ID {
					found = true
					c.Ready = tempController.Ready
					break
				}
			}
			if found == false {
				controllers = append(controllers, tempController)
			}
			logToConsole("Done")
			d.Ack(false)
		}
	}()

	go func() {
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
			logger.Println("Exiting...")
			os.Exit(0)
		case "status":
			logger.Println("Running...")
		case "help":
			fallthrough
		default:
			logger.Println("Help: ")
			logger.Println("   status - Check the status of the world")
			logger.Println("   exit - Exit the App")
		}
		goto ReadCommand
	}()

	// TEMP: Start trigger
	go processTrigger()

	<-forever

	logger.Println("done")
}
