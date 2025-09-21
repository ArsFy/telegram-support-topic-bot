package database

import "time"

type TopicType string

const (
	TopicTypeEmail    TopicType = "email"
	TopicTypeAccount  TopicType = "account"
	TopicTypeTelegram TopicType = "telegram"
)

type Topic struct {
	ID        string    `db:"id" json:"id"`
	Type      TopicType `db:"type" json:"type"`
	Target    string    `db:"target" json:"target"`
	TopicID   int       `db:"topic_id" json:"topic_id"`
	Subject   *string   `db:"subject" json:"subject"`
	MessageID *string   `db:"message_id" json:"message_id"`
	CreatedAt int64     `db:"created_at" json:"created_at"`
}

func GetTopicByTypeTarget(typ TopicType, target string) (*Topic, error) {
	var topic Topic
	err := DB.Get(&topic, "SELECT * FROM topics WHERE type = ? AND target = ? LIMIT 1", typ, target)
	if err != nil {
		return nil, err
	}
	return &topic, nil
}

func GetTopicByTopicID(topicID int) (*Topic, error) {
	var topic Topic
	err := DB.Get(&topic, "SELECT * FROM topics WHERE topic_id = ? LIMIT 1", topicID)
	if err != nil {
		return nil, err
	}
	return &topic, nil
}

func CreateTopic(topic *Topic) error {
	_, err := DB.Exec("INSERT INTO topics (type, target, topic_id, subject, message_id, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		topic.Type, topic.Target, topic.TopicID, topic.Subject, topic.MessageID, time.Now().Unix())
	return err
}

func UpdateTopicSubjectMessageID(topicID int, subject, messageID string) error {
	_, err := DB.Exec("UPDATE topics SET subject = ?, message_id = ? WHERE topic_id = ?",
		subject, messageID, topicID)
	return err
}
