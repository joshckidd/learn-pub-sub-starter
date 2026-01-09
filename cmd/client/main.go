package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	const connectionString string = "amqp://guest:guest@localhost:5672/"
	fmt.Println("Starting Peril server...")

	con, err := amqp.Dial(connectionString)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer con.Close()

	fmt.Println("Connection successful!")

	user, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	name := fmt.Sprintf("%s.%s", routing.PauseKey, user)
	_, _, err = pubsub.DeclareAndBind(con, routing.ExchangePerilDirect, name, routing.PauseKey, pubsub.Transient)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	fmt.Println("Stopping Peril server...")
}
