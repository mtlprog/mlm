package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	PostgresDSN             string
	TelegramToken           string
	Address                 string
	Seed                    string
	ReportToChatID          int64
	ReportToMessageThreadID int64
}

func Get() *Config {
	_ = godotenv.Load()

	reportToChatID, _ := strconv.ParseInt(os.Getenv("REPORT_TO_CHAT_ID"), 10, 64)
	reportToMessageThreadID, _ := strconv.ParseInt(os.Getenv("REPORT_TO_MESSAGE_THREAD_ID"), 10, 64)

	return &Config{
		PostgresDSN:             os.Getenv("POSTGRES_DSN"),
		TelegramToken:           os.Getenv("TELEGRAM_TOKEN"),
		Address:                 os.Getenv("STELLAR_ADDRESS"),
		Seed:                    os.Getenv("STELLAR_SEED"),
		ReportToChatID:          reportToChatID,
		ReportToMessageThreadID: reportToMessageThreadID,
	}
}
