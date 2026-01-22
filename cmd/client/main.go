package main

import (
	"fmt"
	"os"
	"strconv"

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
	err = pubsub.SubscribeJSON(con, routing.ExchangePerilTopic, moveKeyName, "army_moves.*", pubsub.Transient, handlerMove(gs, ch))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = pubsub.SubscribeJSON(con, routing.ExchangePerilTopic, "war", "war.*", pubsub.Durable, handlerWar(gs, ch))
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
			if len(words) > 1 {
				x, err := strconv.Atoi(words[1])
				if err != nil {
					fmt.Println("Invalid spam command.")
				} else {
					for range x {
						ml := gamelogic.GetMaliciousLog()
						pubsub.PublishGameLog(ch, gs.Player.Username, ml)
					}
				}
			} else {
				fmt.Println("Invalid spam command.")
			}
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

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(move gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		mo := gs.HandleMove(move)

		switch mo {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			key := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, move.Player.Username)
			err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, key, gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.GetPlayerSnap(),
			})
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rw)

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeYouWon:
			logMess := fmt.Sprintf("%s won a war against %s", winner, loser)
			err := pubsub.PublishGameLog(ch, gs.Player.Username, logMess)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeOpponentWon:
			logMess := fmt.Sprintf("%s won a war against %s", winner, loser)
			err := pubsub.PublishGameLog(ch, gs.Player.Username, logMess)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			logMess := fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			err := pubsub.PublishGameLog(ch, gs.Player.Username, logMess)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			fmt.Println("Error processing war outcome.")
			return pubsub.NackDiscard
		}
	}
}
