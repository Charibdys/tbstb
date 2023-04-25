package main

import (
	"fmt"
	"os"

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

	bh.HandleMessage(func(bot *telego.Bot, message telego.Message) {
		echoMessage(bot, message)
	}, th.AnyMessage())

	bh.Start()
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
