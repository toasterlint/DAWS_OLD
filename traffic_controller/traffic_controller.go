package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/common"
	. "github.com/toasterlint/DAWS/world_controller/dao"
	worldModels "github.com/toasterlint/DAWS/world_controller/models"
)

var settings worldModels.Settings
var conn *amqp.Connection
var ch *amqp.Channel
var worldtrafficq, trafficjobq amqp.Queue
var msgs <-chan amqp.Delivery
var dao = WorldDAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}
var lastTime time.Time

func connectQueues() {
	var err error
	conn, err = amqp.Dial("amqp://guest:guest@rabbitmq.daws.xyz:5672/")
	FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err = conn.Channel()
	FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	worldtrafficq, err = ch.QueueDeclare(
		"world_traffic_queue", //name
		true,  // durable
		false, //delete when unused
		false, //exclusive
		false, //no-wait
		nil,   //args
	)
	FailOnError(err, "Failed to declase a queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefecth size
		false, // global
	)
	FailOnError(err, "Failed to set QoS")

	msgs, err = ch.Consume(
		worldtrafficq.Name, // queue
		"",                 // consumer
		false,              // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	FailOnError(err, "Failed to register a consumer")
}

func processMsgs() {
	var err error
	for d := range msgs {
		bodyString := string(d.Body[:])
		LogToConsole("Received a message: " + bodyString)
		worldMsg := worldModels.WorldTrafficQueueMessage{}
		json.Unmarshal(d.Body, &worldMsg)
		// Need to use lastTime since settings.LastTime is a string and we need to do time math
		timeLayout := "2006-01-02 15:04:05"
		lastTime, err = time.Parse(timeLayout, settings.LastTime)
		FailOnError(err, "issue converting times")
	}
}

func main() {

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	fmt.Printf("Ready")
	<-forever
}
