package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	database "github.com/Charibdys/tbstb/database"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type RelayParams struct {
	Text      string
	Media     *string
	Entities  []telego.MessageEntity
	Users     []int64
	Reply     map[int64]int
	ParseMode string
	Message   *telego.Message
}

type TBSTBBot struct {
	*telego.Bot
	User *telego.User
}

func AddedToGroup(bot *TBSTBBot) th.Predicate {
	return func(update telego.Update) bool {
		if update.Message == nil {
			return false
		}

		for _, user := range update.Message.NewChatMembers {
			if user.ID == bot.User.ID {
				return true
			}
		}
		return false
	}
}

func main() {
	botToken := os.Getenv("TOKEN")

	telegoBot, err := telego.NewBot(botToken, telego.WithDiscardLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	botUser, err := telegoBot.GetMe()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	bot := &TBSTBBot{
		telegoBot,
		botUser,
	}

	updates, _ := bot.UpdatesViaLongPolling(nil)

	bh, _ := th.NewBotHandler(bot.Bot, updates)

	defer bh.Stop()

	defer bot.StopLongPolling()

	db := database.Init()

	db.CheckCollections()

	config, err := db.GetConfig()
	if err != nil {
		config = db.HandleConfigError()
	}

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		startCommand(bot, &update, db, config)
	}, th.CommandEqual("start"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		broadcastCommand(bot, &update, db)
	}, th.Union(th.CommandEqual("broadcast"), th.CaptionCommandEqual("broadcast")))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		versionCommand(bot, &update, db)
	}, th.CommandEqual("version"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		privacyPolicyCommand(bot, &update)
	}, th.CommandEqual("privacy"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		closeCommand(bot, &update, db)
	}, th.CommandEqual("close"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		reopenCommand(bot, &update, db)
	}, th.CommandEqual("reopen"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		assignCommand(bot, &update, db)
	}, th.CommandEqual("assign"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		roleCommand(bot, &update, db)
	}, th.CommandEqual("role"))

	bh.Handle(func(telegoBot *telego.Bot, update telego.Update) {
		registerGroup(bot, &update, db, config)
	}, AddedToGroup(bot))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		assignToTicket(bot, &query, db)
	}, th.CallbackDataPrefix("assign_user="))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		nextAssignPage(bot, &query, db)
	}, th.CallbackDataPrefix("next_assign_page="))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		prevAssignPage(bot, &query, db)
	}, th.CallbackDataPrefix("prev_assign_page="))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		cancelAssign(bot, &query)
	}, th.CallbackDataEqual("cancel_assign"))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		setRole(bot, &query, db, config)
	}, th.CallbackDataPrefix("setRole="))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		cancelSetRole(bot, &query)
	}, th.CallbackDataEqual("cancel_setrole"))

	bh.HandleMessage(func(telegoBot *telego.Bot, message telego.Message) {
		if message.Chat.Type == "group" || message.Chat.Type == "supergroup" {
			groupMessageHandler(bot, &message, db)
		} else {
			privateMessageHandler(bot, &message, db)
		}
	}, th.AnyMessage())

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		newTicket(bot, &query, db)
	}, th.CallbackDataEqual("new_ticket"))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		addToTicket(bot, &query, db)
	}, th.CallbackDataPrefix("ticket="))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		cancelAddToTicket(bot, &query)
	}, th.CallbackDataEqual("cancel_addto"))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		nextPage(bot, &query, db)
	}, th.CallbackDataPrefix("next_page="))

	bh.HandleCallbackQuery(func(telegoBot *telego.Bot, query telego.CallbackQuery) {
		prevPage(bot, &query, db)
	}, th.CallbackDataPrefix("prev_page="))

	bh.Start()

	defer func() {
		if err = db.Client.Disconnect(context.Background()); err != nil {
			panic(err)
		}
	}()
}

func startCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection, config *database.Config) {
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		return
	}

	if db.GetUserCount() != 0 {
		user, _ := db.GetUser(update.Message.From.ID)
		if user != nil {
			_, _ = bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telego.ChatID{ID: user.ID},
				Text:      "You have already started the bot!",
				ParseMode: "HTML",
			})
		} else {
			username := update.Message.From.Username
			name := update.Message.From.FirstName
			if update.Message.From.LastName != "" {
				name = name + " " + update.Message.From.LastName
			}
			db.CreateUser(update.Message.From.ID, username, name, config)
			_, _ = bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telego.ChatID{ID: update.Message.From.ID},
				Text:      "You have started the bot!",
				ParseMode: "HTML",
			})
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

		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: userID},
			Text:      fmt.Sprintf("Welcome, %s! You have been authorized as owner", name),
			ParseMode: "HTML",
		})
	}
}

func registerGroup(bot *TBSTBBot, update *telego.Update, db *database.Connection, config *database.Config) {
	admins, err := bot.GetChatAdministrators(&telego.GetChatAdministratorsParams{
		ChatID: telego.ChatID{ID: update.Message.Chat.ID},
	})
	if err != nil {
		return
	}

	roles := db.GetAllRoles()

	var ownerRoles []int64
	for _, role := range roles {
		if role.RoleType == "owner" {
			ownerRoles = append(ownerRoles, role.ID)
		}
	}

	var ownerInChat bool
	for _, admin := range admins {
		if admin.MemberStatus() == "creator" && slices.Contains(ownerRoles, admin.MemberUser().ID) {
			ownerInChat = true
		}
	}

	if !ownerInChat {
		return
	}

	config.Groups = append(config.Groups, update.Message.Chat.ID)
	db.UpdateConfig(config)
}

func noUser(bot *TBSTBBot, message *telego.Message) {
	if message.Chat.Type == "group" || message.Chat.Type == "supergroup" {
		return
	}

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: message.From.ID},
		Text:      "Please use /start before creating a ticket.",
		ParseMode: "HTML",
	})
}

func groupMessageHandler(bot *TBSTBBot, message *telego.Message, db *database.Connection) {
	user, err := db.GetUser(message.From.ID)
	if err != nil {
		return
	}

	if message.ReplyToMessage == nil {
		return
	}

	if message.ReplyToMessage.From.ID != bot.User.ID {
		return
	}

	var text string
	if message.Text != "" {
		text = message.Text
	} else if message.Caption != "" {
		text = message.Caption
	}

	id, ticket, reply_message := db.GetTicketAndMessage(message.ReplyToMessage.MessageID, user.ID)
	if ticket == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: message.Chat.ID},
			Text:            "This ticket or message does not exist.",
			ReplyParameters: &telego.ReplyParameters{MessageID: message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	if ticket.ClosedBy != nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: user.ID},
			Text:            fmt.Sprintf("Ticket <code>%s</code> is closed.\nPlease select a different ticket or reopen it with /reopen.", id[len(id)-7:]),
			ReplyParameters: &telego.ReplyParameters{MessageID: message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	reply_to := reply_message.GetMessageReceivers()
	id_short := id[len(id)-7:]
	media, uniqueMediaID := getMessageMediaID(message)

	var fmtText string
	var receivers []int64

	role, _ := db.GetRole(user.ID)
	if role != nil {
		fmtText = formatRoleMessage(text, user, role, id_short)
		receivers = db.GetOriginReceivers(&role.ID, ticket.Creator)
	} else {
		fmtText = formatMessage(text, user, id_short)
		if ticket.Assignees != nil {
			receivers = db.GetAssigneeReceivers(ticket.Assignees)
		} else {
			receivers = db.GetRoleReceivers(&ticket.Creator)
		}
	}

	confirmedReceivers := sendMessage(&RelayParams{
		Text:      fmtText,
		Media:     media,
		Users:     receivers,
		Reply:     reply_to,
		ParseMode: "HTML",
		Message:   message,
	}, bot)

	confirmedReceivers = append(confirmedReceivers, database.Receiver{
		MSID:   message.MessageID,
		UserID: user.ID,
	})

	db.AppendMessage(id, &database.Message{
		Sender:        user.ID,
		OriginMSID:    message.MessageID,
		DateSent:      time.Now(),
		Receivers:     confirmedReceivers,
		Text:          &text,
		Media:         media,
		UniqueMediaID: uniqueMediaID,
	})
}

func privateMessageHandler(bot *TBSTBBot, message *telego.Message, db *database.Connection) {
	user, err := db.GetUser(message.From.ID)
	if err != nil {
		noUser(bot, message)
		return
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

	id, ticket, reply_message := db.GetTicketAndMessage(message.ReplyToMessage.MessageID, user.ID)
	if ticket == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: user.ID},
			Text:            "This ticket or message does not exist.",
			ReplyParameters: &telego.ReplyParameters{MessageID: message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	if ticket.ClosedBy != nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: user.ID},
			Text:            fmt.Sprintf("Ticket <code>%s</code> is closed.\nPlease select a different ticket or reopen it with /reopen.", id[len(id)-7:]),
			ReplyParameters: &telego.ReplyParameters{MessageID: message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	reply_to := reply_message.GetMessageReceivers()
	id_short := id[len(id)-7:]
	media, uniqueMediaID := getMessageMediaID(message)

	var fmtText string
	var receivers []int64

	role, _ := db.GetRole(user.ID)
	if role != nil {
		fmtText = formatRoleMessage(text, user, role, id_short)
		receivers = db.GetOriginReceivers(&role.ID, ticket.Creator)
	} else {
		fmtText = formatMessage(text, user, id_short)
		if ticket.Assignees != nil {
			receivers = db.GetAssigneeReceivers(ticket.Assignees)
		} else {
			receivers = db.GetRoleReceivers(&ticket.Creator)
		}
	}

	confirmedReceivers := sendMessage(&RelayParams{
		Text:      fmtText,
		Media:     media,
		Users:     receivers,
		Reply:     reply_to,
		ParseMode: "HTML",
		Message:   message,
	}, bot)

	confirmedReceivers = append(confirmedReceivers, database.Receiver{
		MSID:   message.MessageID,
		UserID: user.ID,
	})

	db.AppendMessage(id, &database.Message{
		Sender:        user.ID,
		OriginMSID:    message.MessageID,
		DateSent:      time.Now(),
		Receivers:     confirmedReceivers,
		Text:          &text,
		Media:         media,
		UniqueMediaID: uniqueMediaID,
	})
}

func formatMessage(text string, user *database.User, ticket string) string {
	if user.Onymity {
		text = fmt.Sprintf("<b>Anonymous</b>, Ticket: <code>%s</code>\n\n", ticket) + text
	} else {
		text = fmt.Sprintf("<b><a href=\"tg://user?id=%d\">%s</a></b>, Ticket: <code>%s</code>\n\n", user.ID, user.Fullname, ticket) + text
	}

	return text
}

func formatRoleMessage(text string, user *database.User, role *database.Role, ticket string) string {
	if role.Onymity == "realname" {
		text = fmt.Sprintf("<b><a href=\"tg://user?id=%d\">%s</a></b>, Ticket: <code>%s</code>\n\n", user.ID, user.Fullname, ticket) + text
	} else if role.Onymity == "pseudonym" {
		text = fmt.Sprintf("<b>%s</b>, Ticket: <code>%s</code>\n\n", role.Name, ticket) + text
	} else {
		text = fmt.Sprintf("<b>Admin</b>, Ticket: <code>%s</code>\n\n", ticket) + text
	}

	return text
}

func sendMessage(params *RelayParams, bot *TBSTBBot) []database.Receiver {
	var receivers []database.Receiver

	for _, roleID := range params.Users {
		msg := relay(roleID, params, bot)
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

func relay(id int64, params *RelayParams, bot *TBSTBBot) *telego.Message {
	var msg *telego.Message
	if params.Media == nil {
		var err error
		msg, err = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: id},
			Text:            params.Text,
			ParseMode:       params.ParseMode,
			Entities:        params.Entities,
			ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
		})
		if err != nil {
			fmt.Printf("%s\n", err)
		}
	} else {
		switch {
		case params.Message.Animation != nil:
			msg, _ = bot.SendAnimation(&telego.SendAnimationParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: params.Text,
				Animation: telego.InputFile{
					FileID: *params.Media,
				},
				ParseMode:       params.ParseMode,
				CaptionEntities: params.Entities,
				ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
			})
		case params.Message.Document != nil:
			msg, _ = bot.SendDocument(&telego.SendDocumentParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: params.Text,
				Document: telego.InputFile{
					FileID: *params.Media,
				},
				ParseMode:       params.ParseMode,
				CaptionEntities: params.Entities,
				ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
			})
		case params.Message.Sticker != nil:
			msg, _ = bot.SendSticker(&telego.SendStickerParams{
				ChatID: telego.ChatID{ID: id},
				Sticker: telego.InputFile{
					FileID: *params.Media,
				},
			})
		case params.Message.Video != nil:
			msg, _ = bot.SendVideo(&telego.SendVideoParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: params.Text,
				Video: telego.InputFile{
					FileID: *params.Media,
				},
				ParseMode:       params.ParseMode,
				CaptionEntities: params.Entities,
				ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
			})
		case params.Message.VideoNote != nil:
			msg, _ = bot.SendVideoNote(&telego.SendVideoNoteParams{
				ChatID: telego.ChatID{ID: id},
				VideoNote: telego.InputFile{
					FileID: *params.Media,
				},
			})
		case params.Message.Audio != nil:
			msg, _ = bot.SendAudio(&telego.SendAudioParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: params.Text,
				Audio: telego.InputFile{
					FileID: *params.Media,
				},
				ParseMode:       params.ParseMode,
				CaptionEntities: params.Entities,
				ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
			})
		case params.Message.Photo != nil:
			msg, _ = bot.SendPhoto(&telego.SendPhotoParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: params.Text,
				Photo: telego.InputFile{
					FileID: *params.Media,
				},
				ParseMode:       params.ParseMode,
				CaptionEntities: params.Entities,
				ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
			})
		case params.Message.Voice != nil:
			msg, _ = bot.SendVoice(&telego.SendVoiceParams{
				ChatID:  telego.ChatID{ID: id},
				Caption: params.Text,
				Voice: telego.InputFile{
					FileID: *params.Media,
				},
				ParseMode:       params.ParseMode,
				CaptionEntities: params.Entities,
				ReplyParameters: &telego.ReplyParameters{MessageID: params.Reply[id]},
			})
		default:
			return nil
		}
	}

	return msg
}

func noReply(bot *TBSTBBot, original_message int, ticket_ids []string, user *database.User) {
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
				tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_addto"),
			),
		)
	} else {
		markup = tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Create New Ticket").WithCallbackData("new_ticket"),
			),
			tu.InlineKeyboardRow(ticket_options...),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_addto"),
			),
		)
	}

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:          telego.ChatID{ID: user.ID},
		Text:            text,
		ReplyParameters: &telego.ReplyParameters{MessageID: original_message},
		ReplyMarkup:     markup,
		ParseMode:       "HTML",
	})
}

func newTicket(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	var query_msg *telego.Message
	var reply_to *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
		reply_to = query_msg.ReplyToMessage
	}

	user, err := db.GetUser(reply_to.From.ID)
	if err != nil {
		noUser(bot, reply_to)
		return
	}

	var text string
	if reply_to.Text != "" {
		text = reply_to.Text
	} else if reply_to.Caption != "" {
		text = reply_to.Caption
	}

	media, media_unique := getMessageMediaID(reply_to)

	id, id_short, ticket := db.CreateTicket(reply_to.From.ID, reply_to.MessageID, &text, media, media_unique)

	var fmtText string
	role, _ := db.GetRole(user.ID)
	if role != nil {
		fmtText = formatRoleMessage(text, user, role, id_short)
	} else {
		fmtText = formatMessage(text, user, id_short)
	}

	receivers := db.GetRoleReceivers(&user.ID)
	receivers = append(receivers, db.GetGroupReceivers()...)

	confirmedReceivers := sendMessage(&RelayParams{
		Text:      fmtText,
		Media:     media,
		Users:     receivers,
		ParseMode: "HTML",
		Message:   reply_to,
	}, bot)

	confirmedReceivers = append(confirmedReceivers, database.Receiver{
		MSID:   reply_to.MessageID,
		UserID: user.ID,
	})

	ticket.Messages[0].Receivers = confirmedReceivers

	db.UpdateTicket(id, ticket)

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            fmt.Sprintf("Created ticket %s", id_short),
	})

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        fmt.Sprintf("Created ticket <code>%s</code>.\nYour ticket will be addressed shortly.", id_short),
		ParseMode:   "HTML",
		ReplyMarkup: nil,
	})
}

func cancelAddToTicket(bot *TBSTBBot, query *telego.CallbackQuery) {
	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            "Canceled adding message to ticket",
	})

	bot.DeleteMessage(&telego.DeleteMessageParams{
		ChatID:    telego.ChatID{ID: query.From.ID},
		MessageID: query_msg.MessageID,
	})
}

func addToTicket(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	var query_msg *telego.Message
	var reply_to *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
		reply_to = query_msg.ReplyToMessage
	}

	user, err := db.GetUser(reply_to.From.ID)
	if err != nil {
		noUser(bot, reply_to)
		return
	}

	ticketID := strings.Split(query.Data, "=")[1]

	ticket, err := db.GetTicket(ticketID)
	if err != nil {
		return
	}

	if ticket.ClosedBy != nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: user.ID},
			Text:            fmt.Sprintf("Ticket <code>%s</code> is closed.\nPlease select a different ticket or reopen it with /reopen.", ticketID[len(ticketID)-7:]),
			ReplyParameters: &telego.ReplyParameters{MessageID: query_msg.MessageID},
			ParseMode:       "HTML",
		})
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}

	var text string
	if reply_to.Text != "" {
		text = reply_to.Text
	} else if reply_to.Caption != "" {
		text = reply_to.Caption
	}

	media, media_unique := getMessageMediaID(reply_to)

	var fmtText string
	role, _ := db.GetRole(user.ID)
	if role != nil {
		fmtText = formatRoleMessage(text, user, role, ticketID[len(ticketID)-7:])
	} else {
		fmtText = formatMessage(text, user, ticketID[len(ticketID)-7:])
	}

	var receivers []int64
	if ticket.Assignees != nil {
		receivers = db.GetAssigneeReceivers(ticket.Assignees)
	} else {
		receivers = db.GetRoleReceivers(&user.ID)
	}

	confirmedReceivers := sendMessage(&RelayParams{
		Text:      fmtText,
		Media:     media,
		Users:     receivers,
		ParseMode: "HTML",
		Message:   reply_to,
	}, bot)

	confirmedReceivers = append(confirmedReceivers, database.Receiver{
		MSID:   reply_to.MessageID,
		UserID: user.ID,
	})

	db.AppendMessage(ticketID, &database.Message{
		Sender:        user.ID,
		OriginMSID:    query_msg.MessageID,
		DateSent:      time.Now(),
		Receivers:     confirmedReceivers,
		Text:          &text,
		Media:         media,
		UniqueMediaID: media_unique,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            fmt.Sprintf("Added message to ticket %s", ticketID[len(ticketID)-7:]),
	})

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        fmt.Sprintf("Added message to ticket <code>%s</code>.\nYour message will be addressed shortly.", ticketID[len(ticketID)-7:]),
		ParseMode:   "HTML",
		ReplyMarkup: nil,
	})
}

func broadcastCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection) {
	role, err := db.GetRole(update.Message.From.ID)
	if err != nil {
		noUser(bot, update.Message)
		return
	}
	if role.RoleType != "owner" {
		return
	}

	var text string
	if update.Message.Caption == "" {
		text = update.Message.Text
	} else {
		text = update.Message.Caption
	}

	var chatID int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		chatID = update.Message.Chat.ID
	} else {
		chatID = role.ID
	}

	arg := strings.SplitN(text, " ", 2)
	if len(arg) != 2 {
		if update.Message.Caption != "" {
			text = ""
		} else {
			_, _ = bot.SendMessage(&telego.SendMessageParams{
				ChatID:          telego.ChatID{ID: chatID},
				Text:            "The broadcast command requires input.",
				ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
				ParseMode:       "HTML",
			})

			return
		}
	} else {
		text = arg[1]
	}

	if strings.TrimSpace(text) != text {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: chatID},
			Text:            "Please remove any whitespaces between the command and the text.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})

		return
	}

	users := db.GetBroadcastableUsers(&chatID)
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
	} else if len(update.Message.CaptionEntities) > 1 {
		updatedEntities = update.Message.CaptionEntities[1:]
		offset := update.Message.CaptionEntities[0].Length + 1
		for i := range updatedEntities {
			updatedEntities[i].Offset -= offset
		}
	}

	message_media, _ := getMessageMediaID(update.Message)

	confirmedReceivers := sendMessage(&RelayParams{
		Text:     text,
		Media:    message_media,
		Entities: updatedEntities,
		Users:    *users,
		Message:  update.Message,
	}, bot)

	count := len(confirmedReceivers)

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:          telego.ChatID{ID: chatID},
		Text:            fmt.Sprintf("Success! Sent broadcast to %d users.", count),
		ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
	})
}

func versionCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection) {
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		return
	}

	user, err := db.GetUser(update.Message.From.ID)
	if err != nil {
		noUser(bot, update.Message)
		return
	}

	version := "0.4.0"
	source := "github.com/Charibdys/tbstb"

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:             telego.ChatID{ID: user.ID},
		Text:               fmt.Sprintf("TBSTB v%s ~ <a href=\"%s\">[Source]</a>", version, source),
		ReplyParameters:    &telego.ReplyParameters{MessageID: update.Message.MessageID},
		LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: false},
		ParseMode:          "HTML",
	})
}

func privacyPolicyCommand(bot *TBSTBBot, update *telego.Update) {

	privacy_policy := `
<b>Privacy Policy</b>

<i>Last updated: July 03, 2024</i>

The Privacy Policy details what data the SOFTWARE stores, how the SOFTWARE collects it, and for what purpose it is used for.
For the purpose of this Privacy Policy, the terms 'SOFTWARE' refers to the program this Telegram Third Party App uses, 'HOST' refers to the party that is running the SOFTWARE and supplying its service to you, and 'USER' refers to you, the receiver of the service.

Data obtained from the USER is not used to show advertisements.
Only the necessary data from the USER is stored to provide a feature-rich ticketing support system.

Data from the USER is collected and processed for the legitimate purposes of providing services to the USER.

<b><u>The following USER data is collected:</u></b>
<i>Telegram provided unique user ID</i>
This ID belongs to Telegram and not to the USER, and is used to identify the USER's account within the SOFTWARE.
The HOST is not obligated to delete this data from the database, as this would conflict with the legitimate services of the SOFTWARE.
			
<i>Telegram profile information</i>
This includes the full name the USER sets in the Telegram account profile.
This information is updated with every USER message processed by the SOFTWARE.
			
<i>USER message content</i>
The message ID of a USER's message is stored for future processing. This message ID is tied to the USER's Telegram provided unique user ID.
The message media id, unique media ID, sent day, and/or text/caption will be stored for future processing.

<b><u>How USER data is collected and used:</u></b>

USER data is collected and processed by the SOFTWARE with each message or command voluntarily given to the SOFTWARE.

USER message content is voluntarily given to the SOFTWARE. The SOFTWARE will process the message and send a copy to the applicable users of the Telegram Third Party App.

USER data may be collected for anonymized statistics, which can be made publically available or used for internal analysis.

<b><u>USER Data Retention, Deletion, and Security:</u></b>

USER data is retained as long as the HOST deems necessary, to the extent that is required to comply with legal obligations and legitmate services of the SOFTWARE.

The USER has a right to request assistance in deleting the USER's data by contacting the HOST.
The HOST may need to retain certain USER information when there is a legitimate reason to do so.

The HOST is responsible for best effort in securing USER data and engaging with it securely.

<b><u>Privacy Policy Changes, Contacts, and Consent:</u></b>

Changes to the Privacy Policy may be made at any time, and will be announced by the HOST.

This Telegram Third Party App can be used to contact the HOST.

The USER consents to the conditions of the Privacy Policy with the USER's continued use of this Telegram Third Party App.`

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:             telego.ChatID{ID: update.Message.From.ID},
		Text:               privacy_policy,
		ReplyParameters:    &telego.ReplyParameters{MessageID: update.Message.MessageID},
		LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: false},
		ParseMode:          "HTML",
	})
}

func closeCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection) {
	reply_to := update.Message.ReplyToMessage
	if reply_to == nil {
		if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
			return
		}
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: update.Message.From.ID},
			Text:            "Please reply to a message to use this command.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}
	_, err := db.GetUser(update.Message.From.ID)
	if err != nil {
		noUser(bot, update.Message)
		return
	}
	role, err := db.GetRole(update.Message.From.ID)
	if err != nil {
		return
	}

	if !(role.RoleType == "owner" || role.RoleType == "admin") {
		return
	}

	var chatID int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		chatID = update.Message.Chat.ID
	} else {
		chatID = role.ID
	}

	id, id_short, ticket := db.GetTicketFromMSID(reply_to.MessageID, role.ID)
	if ticket == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: chatID},
			Text:            "This ticket or message does not exist.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	ticket.ClosedBy = &role.ID

	closed_time := time.Now()
	ticket.DateClosed = &closed_time

	db.UpdateTicket(id, ticket)

	text := fmt.Sprintf("Ticket <code>%s</code> has been closed.", id_short)

	var receivers []int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		receivers = append(receivers, chatID, ticket.Creator)
	} else {
		receivers = db.GetOriginReceivers(&role.ID, ticket.Creator)
	}

	sendMessage(&RelayParams{
		Text:      text,
		Media:     nil,
		Users:     receivers,
		Reply:     nil,
		ParseMode: "HTML",
		Message:   update.Message,
	}, bot)
}

func reopenCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection) {
	reply_to := update.Message.ReplyToMessage
	if reply_to == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: update.Message.From.ID},
			Text:            "Please reply to a message to use this command.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}
	user, err := db.GetUser(update.Message.From.ID)
	if err != nil {
		noUser(bot, update.Message)
		return
	}

	// if user.CanReopen == false {
	// 	return
	// }

	var chatID int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		chatID = update.Message.Chat.ID
	} else {
		chatID = user.ID
	}

	id, id_short, ticket := db.GetTicketFromMSID(reply_to.MessageID, user.ID)
	if ticket == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: user.ID},
			Text:            "This ticket or message does not exist.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	ticket.ClosedBy = nil
	ticket.DateClosed = nil

	db.UpdateTicket(id, ticket)

	var text string
	if user.Onymity {
		text = fmt.Sprintf("Ticket <code>%s</code> has been reopenned by Anon.", id_short)
	} else {
		text = fmt.Sprintf("Ticket <code>%s</code> has been reopenned by %s.", id_short, user.Fullname)
	}

	var receivers []int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		receivers = append(receivers, chatID, ticket.Creator)
	} else {
		receivers = db.GetOriginReceivers(&user.ID, ticket.Creator)
	}

	sendMessage(&RelayParams{
		Text:      text,
		Media:     nil,
		Users:     receivers,
		Reply:     nil,
		ParseMode: "HTML",
		Message:   update.Message,
	}, bot)
}

func assignCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection) {
	reply_to := update.Message.ReplyToMessage
	if reply_to == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: update.Message.From.ID},
			Text:            "Please reply to a message to use this command.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})
		return
	}
	_, err := db.GetUser(update.Message.From.ID)
	if err != nil {
		noUser(bot, update.Message)
		return
	}
	role, err := db.GetRole(update.Message.From.ID)
	if err != nil {
		return
	}

	if !(role.RoleType == "owner" || role.RoleType == "admin") {
		return
	}

	var chatID int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		chatID = update.Message.Chat.ID
	} else {
		chatID = role.ID
	}

	roles := db.GetAllRoles()

	text := "Which user would you like to assign this ticket to?\n\n" +
		"<b>Available Users</b>:\n\n"

	var role_options []telego.InlineKeyboardButton

	if roles != nil {
		limit := 5
		role_count := len(roles)

		if role_count <= 5 {
			limit = role_count
		}

		for i, role := range roles[:limit] {
			// TODO: Add check for groups; use an anonymous identifier then
			text += fmt.Sprintf("<b>%d.</b> %s\n", i+1, role.Name)

			role_options = append(
				role_options,
				tu.InlineKeyboardButton(fmt.Sprintf("%d", i+1)).WithCallbackData(fmt.Sprintf("assign_user=%d:%d", role.ID, reply_to.MessageID)),
			)
		}

		if role_count > 5 {
			role_options = append(role_options, tu.InlineKeyboardButton("➡️").WithCallbackData(fmt.Sprintf("next_assign_page=2:%d", reply_to.MessageID)))
		}
	}

	if role_options == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: chatID},
			Text:            "There were no roles found to assign this ticket to.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})

		return
	}

	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(role_options...),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_assign"),
		),
	)

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:          telego.ChatID{ID: chatID},
		Text:            text,
		ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
		ReplyMarkup:     markup,
		ParseMode:       "HTML",
	})
}

func roleCommand(bot *TBSTBBot, update *telego.Update, db *database.Connection) {
	role, err := db.GetRole(update.Message.From.ID)
	if err != nil {
		noUser(bot, update.Message)
		return
	}
	if role.RoleType != "owner" {
		return
	}

	var text string
	if update.Message.Caption == "" {
		text = update.Message.Text
	} else {
		text = update.Message.Caption
	}

	var chatID int64
	if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
		chatID = update.Message.Chat.ID
	} else {
		chatID = role.ID
	}

	args := strings.SplitN(text, " ", 2)
	if len(args) != 2 {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: chatID},
			Text:            "The role command requires either a user ID or name.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})

		return
	} else {
		text = args[1]
	}

	if strings.TrimSpace(text) != text {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: chatID},
			Text:            "Please remove any whitespaces between the command and the text.",
			ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
			ParseMode:       "HTML",
		})

		return
	}

	var updated_user *database.User
	var identifier string

	id, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		text, _ = strings.CutPrefix(text, "@")
		updated_user, _ = db.GetUserByName(text)
		if updated_user == nil {
			_, _ = bot.SendMessage(&telego.SendMessageParams{
				ChatID:          telego.ChatID{ID: chatID},
				Text:            fmt.Sprintf("User with the username @%s has not started the bot and cannot be promoted.", text),
				ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
				ParseMode:       "HTML",
			})

			return
		}

		if updated_user.ID == role.ID {
			_, _ = bot.SendMessage(&telego.SendMessageParams{
				ChatID:          telego.ChatID{ID: chatID},
				Text:            "You cannot change your own role as an owner!",
				ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
				ParseMode:       "HTML",
			})

			return
		}

		identifier = fmt.Sprintf("<b><a href=\"tg://user?id=%d\">%s</a></b>", updated_user.ID, updated_user.Fullname)
	} else {
		updated_user, _ = db.GetUser(id)
		if updated_user == nil {
			updated_user = &database.User{
				ID: id,
			}

			identifier = fmt.Sprintf("<b>%d</b>", updated_user.ID)
		} else {
			if updated_user.ID == role.ID {
				_, _ = bot.SendMessage(&telego.SendMessageParams{
					ChatID:          telego.ChatID{ID: chatID},
					Text:            "You cannot change your own role as an owner!",
					ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
					ParseMode:       "HTML",
				})

				return
			}

			identifier = fmt.Sprintf("<b><a href=\"tg://user?id=%d\">%s</a></b>", updated_user.ID, updated_user.Fullname)
		}
	}

	// Query invoker for which role to promote to
	query := fmt.Sprintf("Which role should %s be set to?", identifier)

	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Admin").WithCallbackData(fmt.Sprintf("setRole=%d:%s:admin", updated_user.ID, updated_user.Fullname)),
			tu.InlineKeyboardButton("Moderator").WithCallbackData(fmt.Sprintf("setRole=%d:%s:mod", updated_user.ID, updated_user.Fullname)),
			tu.InlineKeyboardButton("User").WithCallbackData(fmt.Sprintf("setRole=%d:%s:user", updated_user.ID, updated_user.Fullname)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_setrole"),
		),
	)

	_, _ = bot.SendMessage(&telego.SendMessageParams{
		ChatID:          telego.ChatID{ID: chatID},
		Text:            query,
		ReplyParameters: &telego.ReplyParameters{MessageID: update.Message.MessageID},
		ReplyMarkup:     markup,
		ParseMode:       "HTML",
	})
}

func assignToTicket(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	var query_msg *telego.Message
	var reply_to *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
		reply_to = query_msg.ReplyToMessage
	}

	_, err := db.GetUser(reply_to.From.ID)
	if err != nil {
		noUser(bot, reply_to)
		return
	}

	role, err := db.GetRole(reply_to.From.ID)
	if err != nil {
		return
	}

	// TODO: Limit assign to the user that invoked the commmand
	data := strings.Split(query.Data, "=")[1]
	parameters := strings.Split(data, ":")

	userID, err := strconv.ParseInt(parameters[0], 10, 64)
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}
	msid, err := strconv.Atoi(parameters[1])
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}

	assignee, err := db.GetRole(userID)
	if err != nil {
		return
	}

	if assignee.RoleType == "owner" && role.RoleType == "admin" {
		return
	}

	id, id_short, ticket := db.GetTicketFromMSID(msid, reply_to.From.ID)
	if ticket == nil {
		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: role.ID},
			Text:            "This ticket or message does not exist.",
			ReplyParameters: &telego.ReplyParameters{MessageID: query_msg.MessageID},
			ParseMode:       "HTML",
		})
		return
	}

	ticket.Assignees = append(ticket.Assignees, int64(userID))

	db.UpdateTicket(id, ticket)

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            fmt.Sprintf("Assigned %s to ticket %s", assignee.Name, id_short),
	})

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        fmt.Sprintf("Assigned %s to ticket <code>%s</code>.", assignee.Name, id_short),
		ParseMode:   "HTML",
		ReplyMarkup: nil,
	})
}

func cancelAssign(bot *TBSTBBot, query *telego.CallbackQuery) {
	// TODO: Limit cancelling to the user that invoked the command
	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            "Canceled assigning ticket to user",
	})

	bot.DeleteMessage(&telego.DeleteMessageParams{
		ChatID:    telego.ChatID{ID: query.From.ID},
		MessageID: query_msg.MessageID,
	})
}

func setRole(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection, config *database.Config) {
	var query_msg *telego.Message
	var reply_to *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
		reply_to = query_msg.ReplyToMessage
	}

	_, err := db.GetUser(reply_to.From.ID)
	if err != nil {
		noUser(bot, reply_to)
		return
	}

	role, err := db.GetRole(reply_to.From.ID)
	if err != nil {
		return
	}

	if role.RoleType != "owner" {
		return // Not authorized
	}

	// TODO: Limit setting role to the user that invoked the commmand
	data := strings.Split(query.Data, "=")[1]
	parameters := strings.Split(data, ":")

	userID, err := strconv.ParseInt(parameters[0], 10, 64)
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}
	user_name := parameters[1]
	update_role := parameters[2]

	var identifier string
	if user_name != "" {
		identifier = fmt.Sprintf("<b><a href=\"tg://user?id=%d\">%s</a></b>", userID, user_name)
	} else {
		identifier = fmt.Sprintf("<b>%d</b>", userID)
	}

	target_role, err := db.GetRole(userID)
	if err != nil {
		if update_role == "user" {
			bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            "This user's role is unchanged.",
			})

			return
		}

		db.CreateRole(userID, user_name, update_role, config)

		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            fmt.Sprintf("Promoted user to %s", update_role),
		})

		_, _ = bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: userID},
			Text:      fmt.Sprintf("You have been promoted to %s!", update_role),
			ParseMode: "HTML",
		})

		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: query.From.ID},
			MessageID:   query_msg.MessageID,
			Text:        fmt.Sprintf("Promoted user %s to %s.", identifier, update_role),
			ParseMode:   "HTML",
			ReplyMarkup: nil,
		})
	} else {
		if target_role.RoleType == update_role {
			bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            fmt.Sprintf("This user is already a %s.", update_role),
			})

			return
		}

		var promoted bool

		if target_role.RoleType == "admin" && (update_role == "mod" || update_role == "user") {
			promoted = false
		} else if target_role.RoleType == "mod" && update_role == "user" {
			promoted = false
		} else {
			promoted = true
		}

		if promoted {
			target_role.RoleType = update_role

			db.UpdateRole(target_role)

			bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            fmt.Sprintf("Promoted user to %s", update_role),
			})

			_, _ = bot.SendMessage(&telego.SendMessageParams{
				ChatID:    telego.ChatID{ID: userID},
				Text:      fmt.Sprintf("You have been promoted to %s!", update_role),
				ParseMode: "HTML",
			})

			bot.EditMessageText(&telego.EditMessageTextParams{
				ChatID:      telego.ChatID{ID: query.From.ID},
				MessageID:   query_msg.MessageID,
				Text:        fmt.Sprintf("Promoted user %s to %s.", identifier, update_role),
				ParseMode:   "HTML",
				ReplyMarkup: nil,
			})
		} else {
			if update_role == "user" {
				db.DeleteRole(target_role.ID)
			} else {
				target_role.RoleType = update_role

				db.UpdateRole(target_role)
			}

			bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            fmt.Sprintf("Demoted user to %s", update_role),
			})

			bot.DeleteMessage(&telego.DeleteMessageParams{
				ChatID:    telego.ChatID{ID: query.From.ID},
				MessageID: query_msg.MessageID,
			})
		}
	}
}

func cancelSetRole(bot *TBSTBBot, query *telego.CallbackQuery) {
	// TODO: Limit cancelling to the user that invoked the command
	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            "Canceled setting user role.",
	})

	bot.DeleteMessage(&telego.DeleteMessageParams{
		ChatID:    telego.ChatID{ID: query.From.ID},
		MessageID: query_msg.MessageID,
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

func nextPage(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	const page_size = 3

	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

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
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_addto"),
		),
	)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
}

func prevPage(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	const page_size = 3

	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

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
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_addto"),
		),
	)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
}

func nextAssignPage(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	const page_size = 5

	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

	data := strings.Split(query.Data, "=")[1]
	parameters := strings.Split(data, ":")

	page := parameters[0]

	page_number, err := strconv.Atoi(page)
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}
	msid, err := strconv.Atoi(parameters[1])
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}

	roles := db.GetAllRoles()

	roles_page := paginate(page_number, page_size, roles)

	text := "Which user would you like to assign this ticket to?\n\n" +
		"<b>Available Users</b>:\n\n"

	var role_options []telego.InlineKeyboardButton

	role_options = append(
		role_options,
		tu.InlineKeyboardButton("⬅️").WithCallbackData(fmt.Sprintf("prev_assign_page=%d:%d", page_number-1, msid)),
	)

	for i, role := range roles_page {
		// TODO: Add check for groups; use an anonymous identifier then
		text += fmt.Sprintf("<b>%d.</b> %s\n", i+1, role.Name)

		role_options = append(
			role_options,
			tu.InlineKeyboardButton(fmt.Sprintf("%d", i+1)).WithCallbackData(fmt.Sprintf("assign_user=%d:%d", role.ID, msid)),
		)
	}

	if len(roles) > (page_number)*page_size {
		role_options = append(
			role_options,
			tu.InlineKeyboardButton("➡️").WithCallbackData(fmt.Sprintf("next_assign_page=%d:%d", page_number+1, msid)),
		)
	}

	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(role_options...),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_assign"),
		),
	)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
}

func prevAssignPage(bot *TBSTBBot, query *telego.CallbackQuery, db *database.Connection) {
	const page_size = 5

	var query_msg *telego.Message

	switch query.Message.(type) {
	case *telego.InaccessibleMessage:
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "Could not access the query message.",
			ShowAlert:       true,
		})
		return
	case *telego.Message:
		query_msg = query.Message.(*telego.Message)
	}

	data := strings.Split(query.Data, "=")[1]
	parameters := strings.Split(data, ":")

	page := parameters[0]

	page_number, err := strconv.Atoi(page)
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}
	msid, err := strconv.Atoi(parameters[1])
	if err != nil {
		bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
		return
	}

	roles := db.GetAllRoles()

	roles_page := paginate(page_number, page_size, roles)

	text := "Which user would you like to assign this ticket to?\n\n" +
		"<b>Available Users</b>:\n\n"

	var role_options []telego.InlineKeyboardButton

	if page_number != 1 {
		role_options = append(
			role_options,
			tu.InlineKeyboardButton("⬅️").WithCallbackData(fmt.Sprintf("prev_assign_page=%d:%d", page_number-1, msid)),
		)
	}

	for i, role := range roles_page {
		// TODO: Add check for groups; use an anonymous identifier then
		text += fmt.Sprintf("<b>%d.</b> %s\n", i+1, role.Name)

		role_options = append(
			role_options,
			tu.InlineKeyboardButton(fmt.Sprintf("%d", i+1)).WithCallbackData(fmt.Sprintf("assign_user=%d:%d", role.ID, msid)),
		)
	}

	role_options = append(
		role_options,
		tu.InlineKeyboardButton("➡️").WithCallbackData(fmt.Sprintf("next_assign_page=%d:%d", page_number+1, msid)),
	)

	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(role_options...),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Cancel").WithCallbackData("cancel_assign"),
		),
	)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telego.ChatID{ID: query.From.ID},
		MessageID:   query_msg.MessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})

	bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{CallbackQueryID: query.ID})
}

func paginate[T any](page int, page_size int, slice []T) []T {
	if page == 0 {
		return nil
	}

	limit := page_size
	if len(slice) < page*page_size && len(slice) > (page-1)*page_size {
		limit = len(slice) - (page-1)*page_size
	}

	offset := ((page - 1) * page_size)
	var paged_slice []T

	paged_slice = append(paged_slice, slice[offset:offset+(limit)]...)

	return paged_slice
}
