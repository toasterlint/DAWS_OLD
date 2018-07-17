package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/common/dao"
	commonModels "github.com/toasterlint/DAWS/common/models"
	. "github.com/toasterlint/DAWS/common/utils"
)

var settings commonModels.Settings
var conn *amqp.Connection
var ch *amqp.Channel
var cityjobq amqp.Queue
var msgs <-chan amqp.Delivery
var dao = DAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}
var lastTime time.Time
var myself commonModels.Worker

func runConsole() {
	// setup terminal
	reader := bufio.NewReader(os.Stdin)
ReadCommand:
	Logger.Print("Command: ")
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\n")
	switch text {
	case "exit":
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
		cityjobq.Name, // queue
		"",            // consumer
		false,         // auto-ack
		false,         // exclusive
		false,         // no-local
		false,         // no-wait
		nil,           // args
	)
	FailOnError(err, "Failed to register a consumer")
}

func processMsgs() {
	var err error
	for d := range msgs {
		//bodyString := string(d.Body[:])
		//LogToConsole("Received a message: " + bodyString)
		worldMsg := commonModels.WorldCityQueueMessage{}
		json.Unmarshal(d.Body, &worldMsg)
		// Need to use lastTime since settings.LastTime is a string and we need to do time math
		settings = worldMsg.WorldSettings
		FailOnError(err, "issue converting times")
		time.Sleep(time.Millisecond * 1)
		d.Ack(false)
	}
}

func main() {

	id, _ := uuid.NewV4()
	myself = commonModels.Worker{ID: id.String()}

	InitLogger()
	connectQueues()
	defer conn.Close()
	defer ch.Close()
	go processMsgs()

	go runConsole()

	forever := make(chan bool)
	fmt.Println("Ready")
	<-forever
}
