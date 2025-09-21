package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"topic-bot/config"
	"topic-bot/database"
	"topic-bot/email"

	"github.com/emersion/go-imap"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var Bot *bot.Bot

func main() {
	config.Init()
	database.Init()
	email.Init()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	var err error
	Bot, err = bot.New(config.Conf.Token, opts...)
	if err != nil {
		panic(err)
	}

	// Email Callback
	email.InitMonitor(EmailCallback)

	Bot.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message.Chat.ID == config.Conf.ChatID && update.Message.IsTopicMessage && !update.Message.From.IsBot {
		topic, err := database.GetTopicByTopicID(update.Message.MessageThreadID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// Topic not found, ignore
				return
			}
			fmt.Println("Get Topic Error:", err)
			return
		}

		switch topic.Type {
		case database.TopicTypeEmail:
			email.SmtpClient.ReplyEmail(
				topic.Target,
				*topic.Subject,
				*topic.MessageID,
				update.Message.Text+"\n\nSent from "+TelegramName(update.Message.From.FirstName, update.Message.From.LastName),
			)
		}
	}
}

func EmailCallback(msg *imap.Message) {
	var name, address string

	if len(msg.Envelope.From) > 0 {
		from := msg.Envelope.From[0]
		if from.PersonalName != "" {
			name = from.PersonalName
			address = from.MailboxName + "@" + from.HostName
		} else {
			name = from.MailboxName
			address = from.MailboxName + "@" + from.HostName
		}
	}

	var topicId int

	topic, err := database.GetTopicByTypeTarget(database.TopicTypeEmail, address)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			topicId, err = CreateTopic(
				database.TopicTypeEmail,
				address,
				name+" ("+address+")",
				&msg.Envelope.Subject,
				&msg.Envelope.MessageId,
			)
			if err != nil {
				fmt.Println("Create Topic Error:", err)
				return
			}
		} else {
			fmt.Println("Get Topic Error:", err)
			return
		}
	} else {
		topicId = topic.TopicID
	}

	if err := database.UpdateTopicSubjectMessageID(topicId, msg.Envelope.Subject, msg.Envelope.MessageId); err != nil {
		fmt.Println("Update Topic Error:", err)
		return
	}

	body, err := email.ImapClient.GetMessageBody(msg)
	if err != nil {
		fmt.Println("Get Message Body Error:", err)
		return
	}

	if _, err = Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:          config.Conf.ChatID,
		MessageThreadID: topicId,
		Text:            msg.Envelope.Subject + "\n\n" + strings.TrimSpace(strings.Split(body, "> ")[0]),
	}); err != nil {
		fmt.Println("Send Message Error:", err)
		return
	}
}
