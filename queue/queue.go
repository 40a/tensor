package queue

import (
	"github.com/Sirupsen/logrus"
	"github.com/pearsonappeng/tensor/util"
	"github.com/streadway/amqp"
)

const (
	// Ansible is the redis queue which stores jobs
	Ansible = "ansible"
	// Terraform is the redis queue which stores jobs
	Terraform = "terraform"
)

// TestConnect will test the connectivity to rabbitmq
func TestConnect() (err error) {
	conn, err := amqp.Dial(util.Config.RabbitMQ)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return
	}
	defer ch.Close()
	return
}

// Publish publishes a given json message to a given queue
// accepts string and array of bytes and returns a cleanup function and error
func Publish(name string, job []byte) (err error) {
	conn, err := amqp.Dial(util.Config.RabbitMQ)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Queue": name,
			"Error": err.Error(),
		}).Infoln("Could not contact RabbitMQ server")
		return
	}

	defer conn.Close()

	ch, err := conn.Channel()

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Queue": name,
			"Error": err.Error(),
		}).Infoln("Failed to open a channel")
		return
	}

	defer ch.Close()

	q, err := ch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Queue": name,
			"Error": err.Error(),
		}).Infoln("Failed to declare a queue")
		return
	}

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         job,
		})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Queue": name,
			"Error": err.Error(),
		}).Infoln("Failed to publish a message")
		return
	}

	return
}
