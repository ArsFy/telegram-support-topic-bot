package main

func TelegramName(firstName, lastName string) string {
	if lastName != "" {
		return firstName + " " + lastName
	}
	return firstName
}
