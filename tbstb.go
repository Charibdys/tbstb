package main

import (
	"context"
	"fmt"
	"os"
	"strings"

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

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		broadcastCommand(bot, update, db)
	}, th.CommandEqual("broadcast"))

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
			name = name + " " + update.Message.From.LastName
		}

		db.CreateUser(userID, config)
		db.CreateRole(userID, name, "owner", config)

		_, _ = bot.SendMessage(tu.Messagef(
			tu.ID(userID),
			"Welcome, %s! You have been authorized as owner", name,
		))
	}
}

func broadcastCommand(bot *telego.Bot, update telego.Update, db *database.Connection) {
	role, err := db.GetRole(update.Message.From.ID)
	if err != nil {
		return
	}
	if role.RoleType != "owner" {
		return
	}
	arg := strings.SplitN(update.Message.Text, " ", 2)
	if len(arg) != 2 {
		return
	}
	if strings.TrimSpace(arg[1]) != arg[1] {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:           telego.ChatID{ID: role.ID},
			Text:             "Please remove any whitespaces between the command and the text.",
			ReplyToMessageID: update.Message.MessageID,
		})

		return
	}
	users := db.GetBroadcastableUsers()
	if users == nil {
		return
	}

	var updatedEntities []telego.MessageEntity
	if len(update.Message.Entities) > 1 {
		updatedEntities = update.Message.Entities[1:]
		offset := update.Message.Entities[0].Length + 1
		for i := range updatedEntities {
			updatedEntities[i].Offset -= offset
		}
	}

	opts := telego.SendMessageParams{
		Text:     arg[1],
		Entities: updatedEntities,
	}

	var count int32 = 0
	for _, user := range *users {
		if user == role.ID {
			continue
		}
		_, err = bot.SendMessage(
			opts.WithChatID(telego.ChatID{ID: user}),
		)
		if err == nil {
			count++
		}
	}

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:           telego.ChatID{ID: role.ID},
		Text:             fmt.Sprintf("Success! Sent broadcast to %d users.", count),
		ReplyToMessageID: update.Message.MessageID,
	})
}
