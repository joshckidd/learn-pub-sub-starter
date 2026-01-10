package main

import (
	"fmt"
	"os"

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

	ch, err := con.Channel()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println("Connection successful!")

	gamelogic.PrintServerHelp()

	for loop := true; loop; {
		words := gamelogic.GetInput()
		switch words[0] {
		case "pause":
			fmt.Println("Pausing game.")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: true,
			})
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		case "resume":
			fmt.Println("Resuming game.")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: false,
			})
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		case "quit":
			fmt.Println("Stopping Peril server...")
			loop = false
		default:
			fmt.Println("I don't understand that command.")
		}
	}

}
