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

	fmt.Println("Connection successful!")

	user, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	gs := gamelogic.NewGameState(user)
	name := fmt.Sprintf("%s.%s", routing.PauseKey, user)
	err = pubsub.SubscribeJSON(con, routing.ExchangePerilDirect, name, routing.PauseKey, pubsub.Transient, handlerPause(gs))

	for loop := true; loop; {
		words := gamelogic.GetInput()
		switch words[0] {
		case "spawn":
			err = gs.CommandSpawn(words)
			if err != nil {
				fmt.Println(err.Error())
			}
		case "move":
			_, err = gs.CommandMove(words)
			if err != nil {
				fmt.Println(err.Error())
			}
		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			loop = false
		default:
			fmt.Println("I don't understand that command.")
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}
