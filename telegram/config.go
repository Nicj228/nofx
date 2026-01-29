package telegram

import (
	"os"
	"strconv"
	"strings"
)

// LoadConfigFromEnv loads Telegram bot configuration from environment variables
func LoadConfigFromEnv() BotConfig {
	config := DefaultBotConfig()

	// Bot token (required)
	config.Token = os.Getenv("TELEGRAM_BOT_TOKEN")

	// Webhook settings
	if webhook := os.Getenv("TELEGRAM_WEBHOOK_URL"); webhook != "" {
		config.UseWebhook = true
		config.WebhookURL = webhook
	}
	if port := os.Getenv("TELEGRAM_WEBHOOK_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.WebhookPort = p
		}
	}

	// Allowed users (comma-separated list of user IDs)
	if allowedStr := os.Getenv("TELEGRAM_ALLOWED_USERS"); allowedStr != "" {
		for _, idStr := range strings.Split(allowedStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				config.AllowedUserIDs = append(config.AllowedUserIDs, id)
			}
		}
	}

	// Admin users
	if adminStr := os.Getenv("TELEGRAM_ADMIN_USERS"); adminStr != "" {
		for _, idStr := range strings.Split(adminStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				config.AdminUserIDs = append(config.AdminUserIDs, id)
			}
		}
	}

	// Rate limiting
	if rateStr := os.Getenv("TELEGRAM_RATE_LIMIT"); rateStr != "" {
		if rate, err := strconv.Atoi(rateStr); err == nil {
			config.MaxMessagesPerMinute = rate
		}
	}

	// Language
	if lang := os.Getenv("TELEGRAM_LANGUAGE"); lang != "" {
		config.DefaultLanguage = lang
	}

	return config
}
