package main

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/common"
	worldModels "github.com/toasterlint/DAWS/world_controller/models"
)

var settings worldModels.Settings

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq.daws.xyz:5672/")
	FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"world_traffic_job", //name
		true,                // durable
		false,               //delete when unused
		false,               //exclusive
		false,               //no-wait
		nil,                 //args
	)
	FailOnError(err, "Failed to declase a queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefecth size
		false, // global
	)
	FailOnError(err, "Failed to set QoS")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	fmt.Printf("Ready")
	<-forever
}
