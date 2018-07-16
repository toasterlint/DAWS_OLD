package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/mgo.v2/bson"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/common/dao"
	commonModels "github.com/toasterlint/DAWS/common/models"
	. "github.com/toasterlint/DAWS/common/utils"
)

var settings commonModels.Settings
var conn *amqp.Connection
var ch *amqp.Channel
var worldq, worldcityq, cityjobq amqp.Queue
var msgs <-chan amqp.Delivery
var dao = DAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}
var lastTime time.Time
var myself commonModels.Controller

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
		LogToConsole("Notifying World Controller of exit")
		myself.Exit = true
		myself.Ready = false
		tempMsgJSON, _ := json.Marshal(myself)
		err := ch.Publish(
			"",
			worldq.Name,
			false,
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         []byte(tempMsgJSON),
			})
		FailOnError(err, "Failed to notify World Controller of my status")
		Logger.Println("Exiting...")
		os.Exit(0)
	case "status":
		Logger.Println("Waiting for commands from World Controller...")
		tempTrafficJobQ, err := ch.QueueInspect(cityjobq.Name)
		FailOnError(err, "Failed to check Traffic Job Queue")
		tworkers := tempTrafficJobQ.Consumers
		Logger.Printf("Traffic Workers: %d", tworkers)
	case "help":
		fallthrough
	default:
		Logger.Println("Help: ")
		Logger.Println("   status - Check the status of the world")
		Logger.Println("   exit - Exit the App")
	}
	goto ReadCommand
}

func connectQueues() {
	var err error
	conn, err = amqp.Dial("amqp://guest:guest@rabbitmq.daws.xyz:5672/")
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

	worldcityq, err = ch.QueueDeclare(
		"world_city_queue", //name
		true,               // durable
		false,              //delete when unused
		false,              //exclusive
		false,              //no-wait
		nil,                //args
	)
	FailOnError(err, "Failed to declase a queue")

	cityjobq, err = ch.QueueDeclare(
		"city_job_queue", // name
		true,             // durable
		false,            //delete when unused
		false,            // exclusive
		false,            // no wait
		nil,              // arguments
	)
	FailOnError(err, "Failed to declare Traffic Job Queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefecth size
		false, // global
	)
	FailOnError(err, "Failed to set QoS")

	msgs, err = ch.Consume(
		worldcityq.Name, // queue
		"",              // consumer
		false,           // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	FailOnError(err, "Failed to register a consumer")

	publishReady()
}

func publishReady() {
	tempMsgJSON, _ := json.Marshal(myself)
	err := ch.Publish(
		"",
		worldq.Name,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         []byte(tempMsgJSON),
		})
	FailOnError(err, "Failed to notify World Controller of my status")
}

func processMsgs() {
	for d := range msgs {
		bodyString := string(d.Body[:])
		LogToConsole("Received a message: " + bodyString)
		worldMsg := commonModels.WorldCityQueueMessage{}
		json.Unmarshal(d.Body, &worldMsg)
		// Need to use lastTime since settings.LastTime is a string and we need to do time math
		settings = worldMsg.WorldSettings

		//get buildings to queue up workers
		//first check that we actually have a city objectid hex so we don't get a runtime error
		if bson.IsObjectIdHex(worldMsg.City) {
			buildingIDs, err := dao.GetAllBuildingIDs(Mongoid{ID: bson.ObjectIdHex(worldMsg.City)})
			FailOnError(err, "Failed to get Building IDs for city")
			for i := range buildingIDs {
				go publishToWorkQueue(buildingIDs[i].ID)
			}
		}
		d.Ack(false)
		qsize, _ := ch.QueueInspect(cityjobq.Name)
		if qsize.Messages == 0 {
			publishReady()
		}
	}
}

func publishToWorkQueue(bson.ObjectId) {
	err := ch.Publish(
		"",            // exchange
		cityjobq.Name, // routing key
		false,         // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         []byte(""),
		})
	FailOnError(err, "Failed to publish building to job queue")
}

func main() {

	id, _ := uuid.NewV4()
	myself = commonModels.Controller{ID: id.String(), Ready: true, Type: "city", Exit: false}

	InitLogger()
	dao.Connect()
	connectQueues()
	defer conn.Close()
	defer ch.Close()
	go processMsgs()

	go runConsole()

	forever := make(chan bool)
	fmt.Println("Ready")
	<-forever
}
