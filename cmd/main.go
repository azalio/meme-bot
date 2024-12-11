package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/azalio/meme-bot/internal"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv" // Add this for .env file support
)

func main() {
	// 0. Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file (will try environment variables):", err)
	}

	// 1. Initialize Telegram Bot (now checks .env and environment)
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not found in .env or environment variables.")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Error creating Telegram bot:", err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	// Get initial IAM token
	iamToken, err := internal.GetIAMToken()
	if err != nil {
		log.Fatal("Error getting initial IAM token:", err)
	}
	if err := os.Setenv("YANDEX_IAM_TOKEN", iamToken); err != nil {
		log.Fatal("Error setting IAM token env variable:", err)
	}

	// Start goroutine to refresh IAM token every hour.
	go func() {
		for {
			time.Sleep(time.Hour)
			newIAMToken, err := internal.GetIAMToken()
			if err != nil {
				log.Printf("Error refreshing IAM token: %v. Using old token for now.", err)
				continue // Keep using old token if there is an error.
			}
			iamToken = newIAMToken
			if err := os.Setenv("YANDEX_IAM_TOKEN", newIAMToken); err != nil {
				log.Printf("Error updating IAM token env variable: %v", err)
				continue
			}
			log.Println("Successfully refreshed Yandex IAM token")
		}
	}()

	for update := range updates {
		if update.Message == nil {
			continue
		} else {
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "meme":
					handleCommand(bot, update)
				case "help":
					handleCommand(bot, update)
				case "start":
					handleCommand(bot, update)
				default:
					sendMessage(bot, update.Message.Chat.ID, "I don't know that command")
				}
			}
		}

	}
}

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	switch update.Message.Command() {
	case "meme":
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Generating your meme, please wait...")
		statusMsg, err := bot.Send(msg)
		if err != nil {
			log.Printf("Error sending status message: %v", err)
			return
		}

		imageData, err := internal.GenerateImageFromYandexART()
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(update.Message.Chat.ID, statusMsg.MessageID, 
				"Sorry, failed to generate meme: "+err.Error())
			bot.Send(editMsg)
			return
		}

		photo := tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FileBytes{
			Name:  "meme.png",
			Bytes: imageData,
		})
		
		_, err = bot.Send(photo)
		if err != nil {
			log.Printf("Error sending photo: %v", err)
			editMsg := tgbotapi.NewEditMessageText(update.Message.Chat.ID, statusMsg.MessageID,
				"Generated meme but failed to send it: "+err.Error())
			bot.Send(editMsg)
			return
		}

		// Delete the status message
		deleteMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, statusMsg.MessageID)
		bot.Send(deleteMsg)

	case "help":
		sendMessage(bot, update.Message.Chat.ID, "Available commands:\n/meme - Generates a meme\n/start - Starts the bot\n/help - Shows this help message")

	case "start":
		sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf("Hello, %s! I'm a meme bot. Use /meme to generate a meme.", update.Message.From.UserName))

	default: // This shouldn't be reached if the switch in main is correct.
		sendMessage(bot, update.Message.Chat.ID, "Unknown command")

	}
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.Send(msg)
	if err != nil {
		log.Println("Error sending message:", err)
	}
}
