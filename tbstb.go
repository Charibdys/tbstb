package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
		messageHandler(bot, &message, db, config)
	}, th.AnyMessage())

	bh.HandleCallbackQuery(func(bot *telego.Bot, query telego.CallbackQuery) {
		newTicket(bot, &query, db)
	}, th.CallbackDataEqual("new_ticket"))

	bh.HandleCallbackQuery(func(bot *telego.Bot, query telego.CallbackQuery) {
		nextPage(bot, query, db)
	}, th.CallbackDataPrefix("next_page="))

	bh.HandleCallbackQuery(func(bot *telego.Bot, query telego.CallbackQuery) {
		prevPage(bot, query, db)
	}, th.CallbackDataPrefix("prev_page="))

	bh.Start()

	defer func() {
		if err = db.Client.Disconnect(context.Background()); err != nil {
			panic(err)
		}
	}()
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
			username := update.Message.From.Username
			name := update.Message.From.FirstName
			if update.Message.From.LastName != "" {
				name = name + " " + update.Message.From.LastName
			}
			db.CreateUser(update.Message.From.ID, username, name, config)
			_, _ = bot.SendMessage(tu.Message(
				tu.ID(update.Message.From.ID),
				"You have started the bot!",
			))
		}
	} else {
		userID := update.Message.From.ID
		username := update.Message.From.Username
		name := update.Message.From.FirstName
		if update.Message.From.LastName != "" {
			name = name + " " + update.Message.From.LastName
		}

		db.CreateUser(userID, username, name, config)
		db.CreateRole(userID, name, "owner", config)

		_, _ = bot.SendMessage(tu.Messagef(
			tu.ID(userID),
			"Welcome, %s! You have been authorized as owner", name,
		))
	}
}

func messageHandler(bot *telego.Bot, message *telego.Message, db *database.Connection, config *database.Config) {
	user, err := db.GetUser(message.From.ID)
	if err != nil {
		return // TODO: Handle users that do not exist in the database
	}

	if message.ReplyToMessage == nil {
		noReply(bot, message.MessageID, db.GetTicketIDs(user.ID), user)
		return
	}

	var text string
	if message.Text != "" {
		text = message.Text
	} else if message.Caption != "" {
		text = message.Caption
	}

	_, id_short, ticket := db.GetTicketFromMSID(message.ReplyToMessage.MessageID, user.ID)
	reply_message := ticket.GetMessageFromMSID(message.ReplyToMessage.MessageID)
	reply_to := reply_message.GetMessageReceivers()

	media, _ := getMessageMediaID(message)

	new_message := database.Message{
		Sender:     user.ID,
		OriginMSID: message.MessageID,
		DateSent:   time.Now(),
		Receivers:  nil,
		Text:       &text,
		Media:      media,
	}

	var fmt_text string
	role, _ := db.GetRole(user.ID)
	if role != nil {
		fmt_text = formatRoleMessage(text, user, role, id_short)
	} else {
		fmt_text = formatMessage(text, user, id_short)
	}

	receivers := sendMessageToOrigin(fmt_text, media, &ticket.Creator, reply_to, message, bot, db)

	new_message.Receivers = receivers

	ticket.Messages = append(ticket.Messages, new_message)

	db.UpdateTicket(ticket.ID.Hex(), ticket)
}

func formatMessage(text string, user *database.User, ticket string) string {
	// TODO: Replace name with a user mention/link to account if possible

	if user.Onymity {
		text = fmt.Sprintf("<b>Anonymous</b>, Ticket: %s\n\n", ticket[len(ticket)-7:]) + text
	} else {
		text = fmt.Sprintf("<b>%s</b>, Ticket: %s\n\n", user.Name, ticket[len(ticket)-7:]) + text
	}

	return text
}

func formatRoleMessage(text string, user *database.User, role *database.Role, ticket string) string {
	// TODO: Replace name with a user mention/link to account if possible

	if role.Onymity == "anon" {
		text = fmt.Sprintf("<b>Admin</b>, Ticket: %s\n\n", ticket[len(ticket)-7:]) + text
	} else if role.Onymity == "pseudonym" {
		text = fmt.Sprintf("<b>%s</b>, Ticket: %s\n\n", role.Name, ticket[len(ticket)-7:]) + text
	} else {
		text = fmt.Sprintf("<b>%s</b>, Ticket: %s\n\n", user.Name, ticket[len(ticket)-7:]) + text
	}

	return text
}

func sendMessageToRoles(text string, media *string, sender *int64, message *telego.Message, bot *telego.Bot, db *database.Connection) []database.Receiver {
	roles := db.GetRoleIDs(sender)

	var receivers []database.Receiver
	for _, roleID := range roles {
		msg := relay(roleID, text, media, message, bot)
		if msg == nil {
			continue
		}
		receivers = append(receivers, database.Receiver{
			MSID:   msg.MessageID,
			UserID: roleID,
		})
	}

	return receivers
}

func sendMessageToOrigin(text string, media *string, origin *int64, reply_to map[int64]int, message *telego.Message, bot *telego.Bot, db *database.Connection) []database.Receiver {
	users := db.GetRoleIDs(origin)
	users = append(users, *origin)

	var receivers []database.Receiver
	for _, userID := range users {
		msg := relayWithReply(userID, text, media, reply_to[userID], message, bot)
		if msg == nil {
			continue
		}
		receivers = append(receivers, database.Receiver{
			MSID:   msg.MessageID,
			UserID: userID,
		})
	}

	return receivers
}

func relay(id int64, text string, media *string, message *telego.Message, bot *telego.Bot) *telego.Message {
	var msg *telego.Message
	if media == nil {
		var err error
		msg, err = bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: id},
			Text:      text,
			ParseMode: "HTML",
		})
		if err != nil {
			fmt.Printf("%s\n", err)
		}
	} else {
		switch {
		case message.Animation != nil:
			msg, _ = bot.SendAnimation(&telego.SendAnimationParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Animation: telego.InputFile{
					FileID: *media,
				},
				ParseMode: "HTML",
			})
		case message.Document != nil:
			msg, _ = bot.SendDocument(&telego.SendDocumentParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Document: telego.InputFile{
					FileID: *media,
				},
				ParseMode: "HTML",
			})
		case message.Sticker != nil:
			msg, _ = bot.SendSticker(&telego.SendStickerParams{
				ChatID: telego.ChatID{ID: id},
				Sticker: telego.InputFile{
					FileID: *media,
				},
			})
		case message.Video != nil:
			msg, _ = bot.SendVideo(&telego.SendVideoParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Video: telego.InputFile{
					FileID: *media,
				},
				ParseMode: "HTML",
			})
		case message.VideoNote != nil:
			msg, _ = bot.SendVideoNote(&telego.SendVideoNoteParams{
				ChatID: telego.ChatID{ID: id},
				VideoNote: telego.InputFile{
					FileID: *media,
				},
			})
		case message.Audio != nil:
			msg, _ = bot.SendAudio(&telego.SendAudioParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Audio: telego.InputFile{
					FileID: *media,
				},
				ParseMode: "HTML",
			})
		case message.Photo != nil:
			msg, _ = bot.SendPhoto(&telego.SendPhotoParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Photo: telego.InputFile{
					FileID: *media,
				},
				ParseMode: "HTML",
			})
		case message.Voice != nil:
			msg, _ = bot.SendVoice(&telego.SendVoiceParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Voice: telego.InputFile{
					FileID: *media,
				},
				ParseMode: "HTML",
			})
		default:
			return nil
		}
	}

	return msg
}

func relayWithReply(id int64, text string, media *string, reply_to int, message *telego.Message, bot *telego.Bot) *telego.Message {
	var msg *telego.Message
	if media == nil {
		var err error
		msg, err = bot.SendMessage(&telego.SendMessageParams{
			ChatID:           telego.ChatID{ID: id},
			Text:             text,
			ReplyToMessageID: reply_to,
			ParseMode:        "HTML",
		})
		if err != nil {
			fmt.Printf("%s\n", err)
		}
	} else {
		switch {
		case message.Animation != nil:
			msg, _ = bot.SendAnimation(&telego.SendAnimationParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Animation: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
				ParseMode:        "HTML",
			})
		case message.Document != nil:
			msg, _ = bot.SendDocument(&telego.SendDocumentParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Document: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
				ParseMode:        "HTML",
			})
		case message.Sticker != nil:
			msg, _ = bot.SendSticker(&telego.SendStickerParams{
				ChatID: telego.ChatID{ID: id},
				Sticker: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
			})
		case message.Video != nil:
			msg, _ = bot.SendVideo(&telego.SendVideoParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Video: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
				ParseMode:        "HTML",
			})
		case message.VideoNote != nil:
			msg, _ = bot.SendVideoNote(&telego.SendVideoNoteParams{
				ChatID: telego.ChatID{ID: id},
				VideoNote: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
			})
		case message.Audio != nil:
			msg, _ = bot.SendAudio(&telego.SendAudioParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Audio: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
				ParseMode:        "HTML",
			})
		case message.Photo != nil:
			msg, _ = bot.SendPhoto(&telego.SendPhotoParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Photo: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
				ParseMode:        "HTML",
			})
		case message.Voice != nil:
			msg, _ = bot.SendVoice(&telego.SendVoiceParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: text,
				Voice: telego.InputFile{
					FileID: *media,
				},
				ReplyToMessageID: reply_to,
				ParseMode:        "HTML",
			})
		default:
			return nil
		}
	}

	return msg
}

func noReply(bot *telego.Bot, original_message int, ticket_ids []string, user *database.User) {
	text := "No reply found.\n\n" +
		"Would you like to create a new ticket with this message " +
		"or add this message to one of your tickets?\n\n"

	var ticket_options []telego.InlineKeyboardButton
	if ticket_ids != nil {
		limit := 3
		ticket_count := len(ticket_ids)
		if ticket_count <= 3 {
			text += fmt.Sprintf("<b>Your tickets 1-%d</b>\n\n", ticket_count)
			limit = ticket_count
		} else {
			text += fmt.Sprintf("<b>Your tickets 1-3 of %d</b>\n\n", ticket_count)
		}

		for i, id := range ticket_ids[:limit] {
			text += fmt.Sprintf("<b>%d.</b> <code>%s</code>\n", i+1, id[len(id)-7:])
			ticket_options = append(
				ticket_options,
				tu.InlineKeyboardButton(fmt.Sprintf("%d", i+1)).WithCallbackData(fmt.Sprintf("ticket=%s", id)),
			)
		}

		if ticket_count > 3 {
			ticket_options = append(ticket_options, tu.InlineKeyboardButton("➡️").WithCallbackData("next_page=2"))
		}
	}

	var markup *telego.InlineKeyboardMarkup
	if ticket_options == nil {
		markup = tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Create New Ticket").WithCallbackData("new_ticket"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel"),
			),
		)
	} else {
		markup = tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Create New Ticket").WithCallbackData("new_ticket"),
			),
			tu.InlineKeyboardRow(ticket_options...),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel"),
			),
		)
	}

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:           telego.ChatID{ID: user.ID},
		Text:             text,
		ReplyToMessageID: original_message,
		ReplyMarkup:      markup,
		ParseMode:        "HTML",
	})
}

func newTicket(bot *telego.Bot, query *telego.CallbackQuery, db *database.Connection) {
	reply_to := query.Message.ReplyToMessage
	user, err := db.GetUser(reply_to.From.ID)
	if err != nil {
		return // TODO: Handle users that do not exist in the database
	}

	var text string
	if reply_to.Text != "" {
		text = reply_to.Text
	} else if reply_to.Caption != "" {
		text = reply_to.Caption
	}

	media, media_unique := getMessageMediaID(reply_to)

	id, id_short, ticket := db.CreateTicket(reply_to.From.ID, reply_to.MessageID, &text, media, media_unique)

	var fmt_text string
	role, _ := db.GetRole(user.ID)
	if role != nil {
		fmt_text = formatRoleMessage(text, user, role, id_short)
	} else {
		fmt_text = formatMessage(text, user, id_short)
	}

	ticket.Messages[0].Receivers = append(
		ticket.Messages[0].Receivers,
		sendMessageToRoles(fmt_text, media, &user.ID, reply_to, bot, db)...,
	)

	db.UpdateTicket(id, ticket)

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            fmt.Sprintf("Created ticket %s", id_short),
	})
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

func getMessageMediaID(message *telego.Message) (*string, *string) {
	switch {
	case message.Animation != nil:
		return &message.Animation.FileID, &message.Animation.FileUniqueID
	case message.Document != nil:
		return &message.Document.FileID, &message.Document.FileUniqueID
	case message.Sticker != nil:
		return &message.Sticker.FileID, &message.Sticker.FileUniqueID
	case message.Video != nil:
		return &message.Video.FileID, &message.Video.FileUniqueID
	case message.VideoNote != nil:
		return &message.VideoNote.FileID, &message.VideoNote.FileUniqueID
	case message.Audio != nil:
		return &message.Audio.FileID, &message.Audio.FileUniqueID
	case message.Photo != nil:
		largest, width := 0, 0
		for i, photo := range message.Photo {
			if photo.Width > width {
				width = photo.Width
				largest = i
			}
		}
		return &message.Photo[largest].FileID, &message.Photo[largest].FileUniqueID
	case message.Voice != nil:
		return &message.Voice.FileID, &message.Voice.FileUniqueID
	}
	return nil, nil
}

func nextPage(bot *telego.Bot, query telego.CallbackQuery, db *database.Connection) {
	const page_size = 3

	page := strings.Split(query.Data, "=")[1]

	page_number, err := strconv.Atoi(page)
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}

	tickets := db.GetTicketIDs(query.From.ID)

	ticket_page := paginate(page_number, page_size, tickets)

	text := "No reply found.\n\n" +
		"Would you like to create a new ticket with this message " +
		"or add this message to one of your tickets?\n\n"

	if len(ticket_page) != 3 {
		text += fmt.Sprintf("<b>Your tickets %d-%d of %d</b>\n\n", ((page_number-1)*page_size)+1, len(tickets), len(tickets))
	} else {
		text += fmt.Sprintf("<b>Your tickets %d-%d of %d</b>\n\n", ((page_number-1)*page_size)+1, (page_number * page_size), len(tickets))
	}

	var ticket_options []telego.InlineKeyboardButton

	ticket_options = append(
		ticket_options,
		tu.InlineKeyboardButton("⬅️").WithCallbackData(fmt.Sprintf("prev_page=%d", page_number-1)),
	)

	for i, id := range ticket_page {
		text += fmt.Sprintf("<b>%d.</b> <code>%s</code>\n", i+1, id[len(id)-7:])
		ticket_options = append(
			ticket_options,
			tu.InlineKeyboardButton(fmt.Sprintf("%d", i+1)).WithCallbackData(fmt.Sprintf("ticket=%s", id)),
		)
	}

	if len(tickets) > (page_number)*page_size {
		ticket_options = append(
			ticket_options,
			tu.InlineKeyboardButton("➡️").WithCallbackData(fmt.Sprintf("next_page=%d", page_number+1)),
		)
	}

	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Create New Ticket").WithCallbackData("new_ticket"),
		),
		tu.InlineKeyboardRow(ticket_options...),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel"),
		),
	)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query.Message.MessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
}

func prevPage(bot *telego.Bot, query telego.CallbackQuery, db *database.Connection) {
	const page_size = 3

	page := strings.Split(query.Data, "=")[1]

	page_number, err := strconv.Atoi(page)
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}

	tickets := db.GetTicketIDs(query.From.ID)

	ticket_page := paginate(page_number, page_size, tickets)

	text := "No reply found.\n\n" +
		"Would you like to create a new ticket with this message " +
		"or add this message to one of your tickets?\n\n"

	text += fmt.Sprintf("<b>Your tickets %d-%d of %d</b>\n\n", ((page_number-1)*page_size)+1, (page_number * page_size), len(tickets))

	var ticket_options []telego.InlineKeyboardButton

	if page_number != 1 {
		ticket_options = append(
			ticket_options,
			tu.InlineKeyboardButton("⬅️").WithCallbackData(fmt.Sprintf("prev_page=%d", page_number-1)),
		)
	}

	for i, id := range ticket_page {
		text += fmt.Sprintf("<b>%d.</b> <code>%s</code>\n", i+1, id[len(id)-7:])
		ticket_options = append(
			ticket_options,
			tu.InlineKeyboardButton(fmt.Sprintf("%d", i+1)).WithCallbackData(fmt.Sprintf("ticket=%s", id)),
		)
	}

	ticket_options = append(
		ticket_options,
		tu.InlineKeyboardButton("➡️").WithCallbackData(fmt.Sprintf("next_page=%d", page_number+1)),
	)

	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Create New Ticket").WithCallbackData("new_ticket"),
		),
		tu.InlineKeyboardRow(ticket_options...),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel"),
		),
	)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query.Message.MessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
}

func paginate(page int, page_size int, slice []string) []string {
	if page == 0 {
		return nil
	}

	limit := page_size
	if len(slice) < page*page_size && len(slice) > (page-1)*page_size {
		limit = len(slice) - (page-1)*page_size
	}

	offset := ((page - 1) * page_size)
	var paged_slice []string

	paged_slice = append(paged_slice, slice[offset:offset+(limit)]...)

	return paged_slice
}
