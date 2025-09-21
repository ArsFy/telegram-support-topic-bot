package main

import (
	"context"
	"topic-bot/config"
	"topic-bot/database"

	"github.com/go-telegram/bot"
)

func CreateTopic(typ database.TopicType, target, name string, subject, messageId *string) (int, error) {
	data, err := Bot.CreateForumTopic(context.Background(), &bot.CreateForumTopicParams{
		ChatID: config.Conf.ChatID,
		Name:   name,
	})
	if err != nil {
		return -1, err
	}

	if err = database.CreateTopic(&database.Topic{
		Type:      typ,
		Target:    target,
		TopicID:   data.MessageThreadID,
		Subject:   subject,
		MessageID: messageId,
	}); err != nil {
		return -1, err
	}

	return data.MessageThreadID, nil
}
