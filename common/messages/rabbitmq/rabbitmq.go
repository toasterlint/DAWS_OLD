package rabbitmq

import (
	"github.com/streadway/amqp"
	. "github.com/toasterlint/DAWS/common/utils"
)

type Queue struct {
	Name  string
	Queue amqp.Queue
}

// RABBITMQ RabbitMQ object
type RABBITMQ struct {
	Conn             *amqp.Connection
	Ch               *amqp.Channel
	Queues           []Queue
	Msgs             <-chan amqp.Delivery
	ConnectionString string
}

func (m *RABBITMQ) Connect() {
	var err error
	m.Conn, err = amqp.Dial(m.ConnectionString)
	FailOnError(err, "Failed to open connection to RabbitMQ")

	m.Ch, err = m.Conn.Channel()
	FailOnError(err, "Failed to open a channel")

	for i := range m.Queues {
		m.Queues[i].Queue, err = m.Ch.QueueDeclare(
			m.Queues[i].Name, //name
			true,             // durable
			false,            // delete when unused
			false,            // exclusive
			false,            // no-wait
			nil,              // arguments
		)
		FailOnError(err, "Failed to declare queue")
	}
}
