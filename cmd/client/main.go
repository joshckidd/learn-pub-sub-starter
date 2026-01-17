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

	user, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	gs := gamelogic.NewGameState(user)
	pauseKeyName := fmt.Sprintf("%s.%s", routing.PauseKey, user)
	err = pubsub.SubscribeJSON(con, routing.ExchangePerilDirect, pauseKeyName, routing.PauseKey, pubsub.Transient, handlerPause(gs))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	moveKeyName := fmt.Sprintf("%s.%s", "army_moves", user)
	err = pubsub.SubscribeJSON(con, routing.ExchangePerilTopic, moveKeyName, "army_moves.*", pubsub.Transient, handlerMove(gs))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for loop := true; loop; {
		words := gamelogic.GetInput()
		switch words[0] {
		case "spawn":
			err = gs.CommandSpawn(words)
			if err != nil {
				fmt.Println(err.Error())
			}
		case "move":
			move, err := gs.CommandMove(words)
			if err != nil {
				fmt.Println(err.Error())
			}
			err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, moveKeyName, move)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(move gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		mo := gs.HandleMove(move)

		switch mo {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}
