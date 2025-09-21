package config

import (
	"encoding/json"
	"log"
	"os"
)

type ConfigType struct {
	Name string `json:"name"`

	Token  string `json:"token"`
	ChatID int64  `json:"chat_id"`

	// Database
	Database struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}

	// Email
	Email struct {
		SMTP     string `json:"smtp"`
		IMAP     string `json:"imap"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
}

var Conf ConfigType

func Init() {
	file, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalln(err)
	}
	if err := json.Unmarshal(file, &Conf); err != nil {
		log.Fatalln(err)
	}
}
