package config

import (
	"fmt"
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
	SwapPriceThreshold      float64
	AlertMentionUsername    string
}

func (c *Config) Validate() error {
	if c.PostgresDSN == "" {
		return fmt.Errorf("POSTGRES_DSN is required")
	}
	if c.Address == "" {
		return fmt.Errorf("STELLAR_ADDRESS is required")
	}
	if c.Seed == "" {
		return fmt.Errorf("STELLAR_SEED is required")
	}
	return nil
}

func Get() *Config {
	_ = godotenv.Load()

	reportToChatID, _ := strconv.ParseInt(os.Getenv("REPORT_TO_CHAT_ID"), 10, 64)
	reportToMessageThreadID, _ := strconv.ParseInt(os.Getenv("REPORT_TO_MESSAGE_THREAD_ID"), 10, 64)

	swapPriceThreshold, _ := strconv.ParseFloat(os.Getenv("SWAP_PRICE_THRESHOLD"), 64)
	if swapPriceThreshold == 0 {
		swapPriceThreshold = 25.0
	}

	alertMentionUsername := os.Getenv("ALERT_MENTION_USERNAME")
	if alertMentionUsername == "" {
		alertMentionUsername = "xdefrag"
	}

	return &Config{
		PostgresDSN:             os.Getenv("POSTGRES_DSN"),
		TelegramToken:           os.Getenv("TELEGRAM_TOKEN"),
		Address:                 os.Getenv("STELLAR_ADDRESS"),
		Seed:                    os.Getenv("STELLAR_SEED"),
		ReportToChatID:          reportToChatID,
		ReportToMessageThreadID: reportToMessageThreadID,
		SwapPriceThreshold:      swapPriceThreshold,
		AlertMentionUsername:    alertMentionUsername,
	}
}
