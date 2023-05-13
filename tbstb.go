package main

import (
	"context"
	"fmt"
	"os"

	database "github.com/Charibdys/tbstb/database"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func main() {
	botToken := os.Getenv("TOKEN")

	bot, err := telego.NewBot(botToken, telego.WithDiscardLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	updates, _ := bot.UpdatesViaLongPolling(nil)

	bh, _ := th.NewBotHandler(bot, updates)

	defer bh.Stop()

	defer bot.StopLongPolling()

	db := database.Init()

	db.CheckCollections()

	config, err := db.GetConfig()
	if err != nil {
		config = db.HandleConfigError()
	}

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		startCommand(bot, update, db, config)
	}, th.CommandEqual("start"))

	bh.HandleMessage(func(bot *telego.Bot, message telego.Message) {
		echoMessage(bot, message)
	}, th.AnyMessage())

	bh.Start()

	defer func() {
		if err = db.Client.Disconnect(context.Background()); err != nil {
			panic(err)
		}
	}()
}

// echoMessage echos message back to sender
func echoMessage(bot *telego.Bot, message telego.Message) {
	chatID := tu.ID(message.Chat.ID)
	_, _ = bot.CopyMessage(tu.CopyMessage(
		chatID,
		chatID,
		message.MessageID,
	))
}

func startCommand(bot *telego.Bot, update telego.Update, db *database.Connection, config *database.Config) {
	if db.GetUserCount() != 0 {
		user, _ := db.GetUser(update.Message.From.ID)
		if user != nil {
			_, _ = bot.SendMessage(tu.Message(
				tu.ID(user.ID),
				"You have already started the bot!",
			))
		} else {
			db.CreateUser(update.Message.From.ID, config)
			_, _ = bot.SendMessage(tu.Message(
				tu.ID(update.Message.From.ID),
				"You have started the bot!",
			))
		}
	} else {
		userID := update.Message.From.ID
		name := update.Message.From.FirstName
		if len(update.Message.From.LastName) != 0 {
			name = name + "" + update.Message.From.LastName
		}

		db.CreateUser(userID, config)
		db.CreateRole(userID, name, "owner", config)

		_, _ = bot.SendMessage(tu.Messagef(
			tu.ID(userID),
			"Welcome, %s! You have been authorized as owner", name,
		))
	}
}
