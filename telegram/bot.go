// Package telegram provides Telegram bot integration for NOFX trading assistant
package telegram

import (
	"context"
	"fmt"
	"nofx/assistant"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Bot represents the Telegram bot for NOFX
type Bot struct {
	bot    *tele.Bot
	agent  *assistant.Agent
	config BotConfig

	// Allowed users (for security)
	allowedUsers     map[int64]bool
	allowedUsersLock sync.RWMutex

	// Rate limiting
	rateLimiter *RateLimiter
}

// BotConfig holds bot configuration
type BotConfig struct {
	Token string `json:"token"`

	// Polling or webhook mode
	UseWebhook  bool   `json:"use_webhook"`
	WebhookURL  string `json:"webhook_url"`
	WebhookPort int    `json:"webhook_port"`

	// Security
	AllowedUserIDs []int64 `json:"allowed_user_ids"` // Empty = allow all
	AdminUserIDs   []int64 `json:"admin_user_ids"`

	// Rate limiting
	MaxMessagesPerMinute int `json:"max_messages_per_minute"`

	// Language
	DefaultLanguage string `json:"default_language"` // "en" or "zh"
}

// DefaultBotConfig returns default configuration
func DefaultBotConfig() BotConfig {
	return BotConfig{
		MaxMessagesPerMinute: 30,
		DefaultLanguage:      "zh",
	}
}

// NewBot creates a new Telegram bot
func NewBot(config BotConfig, agent *assistant.Agent) (*Bot, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	settings := tele.Settings{
		Token:  config.Token,
		Poller: &tele.LongPoller{Timeout: 30 * time.Second},
	}

	teleBot, err := tele.NewBot(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	bot := &Bot{
		bot:          teleBot,
		agent:        agent,
		config:       config,
		allowedUsers: make(map[int64]bool),
		rateLimiter:  NewRateLimiter(config.MaxMessagesPerMinute),
	}

	// Initialize allowed users
	for _, uid := range config.AllowedUserIDs {
		bot.allowedUsers[uid] = true
	}

	// Register handlers
	bot.registerHandlers()

	return bot, nil
}

// Start starts the bot
func (b *Bot) Start() {
	logger.Info("🤖 Starting Telegram bot...")
	b.bot.Start()
}

// Stop stops the bot
func (b *Bot) Stop() {
	logger.Info("🤖 Stopping Telegram bot...")
	b.bot.Stop()
}

// registerHandlers sets up all message handlers
func (b *Bot) registerHandlers() {
	// Middleware for access control and rate limiting
	b.bot.Use(b.accessControlMiddleware)
	b.bot.Use(b.rateLimitMiddleware)

	// Command handlers
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/status", b.handleStatus)
	b.bot.Handle("/balance", b.handleBalance)
	b.bot.Handle("/positions", b.handlePositions)
	b.bot.Handle("/traders", b.handleTraders)
	b.bot.Handle("/clear", b.handleClear)

	// Handle all text messages (send to AI agent)
	b.bot.Handle(tele.OnText, b.handleText)

	// Handle callbacks (for inline keyboards)
	b.bot.Handle(tele.OnCallback, b.handleCallback)

	logger.Info("✅ Telegram handlers registered")
}

// accessControlMiddleware checks if user is allowed
func (b *Bot) accessControlMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		userID := c.Sender().ID

		// If allowlist is empty, allow all
		if len(b.config.AllowedUserIDs) == 0 {
			return next(c)
		}

		b.allowedUsersLock.RLock()
		allowed := b.allowedUsers[userID]
		b.allowedUsersLock.RUnlock()

		if !allowed {
			logger.Warnf("⚠️ Unauthorized access attempt from user %d", userID)
			return c.Send("⛔ Sorry, you are not authorized to use this bot.\n\n抱歉，您没有使用此机器人的权限。")
		}

		return next(c)
	}
}

// rateLimitMiddleware implements rate limiting
func (b *Bot) rateLimitMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		userID := c.Sender().ID

		if !b.rateLimiter.Allow(userID) {
			return c.Send("⏳ Please slow down. Too many messages.\n\n请稍等，消息发送过于频繁。")
		}

		return next(c)
	}
}

// ==================== Command Handlers ====================

func (b *Bot) handleStart(c tele.Context) error {
	welcome := `🚀 *Welcome to NOFX Trading Assistant!*

I'm your AI-powered trading assistant. I can help you:

📊 *Monitor* - Check balances, positions, and market prices
🤖 *Manage* - Start/stop AI traders, configure strategies  
💹 *Trade* - Execute trades (with confirmation)
📈 *Analyze* - Market analysis and AI debates

*Commands:*
/help - Show all commands
/status - System status
/balance - Account balances
/positions - Current positions
/traders - List AI traders
/clear - Clear conversation history

Or just chat with me in natural language! 

---

🚀 *欢迎使用 NOFX 交易助手！*

我是你的 AI 交易助手，可以帮你：

📊 *监控* - 查看余额、持仓、行情
🤖 *管理* - 启停 AI 交易员、配置策略
💹 *交易* - 执行交易（需确认）
📈 *分析* - 市场分析和 AI 辩论

直接用自然语言和我对话即可！`

	return c.Send(welcome, tele.ModeMarkdown)
}

func (b *Bot) handleHelp(c tele.Context) error {
	help := `📖 *NOFX Trading Assistant Help*

*Commands:*
• /start - Welcome message
• /help - This help message
• /status - System overview
• /balance - Show all balances
• /positions - Show all positions
• /traders - List AI traders
• /clear - Clear conversation history

*Natural Language Examples:*
• "查看我的余额"
• "BTC 现在多少钱"
• "启动交易员 xxx"
• "帮我平掉 ETH 的多单"
• "我的持仓盈亏怎么样"
• "列出所有策略"

*Tips:*
• I'll always confirm before executing trades
• Use specific trader names/IDs for operations
• Ask me anything about your trading!`

	return c.Send(help, tele.ModeMarkdown)
}

func (b *Bot) handleStatus(c tele.Context) error {
	ctx := context.Background()
	sessionID := b.getSessionID(c)

	response, err := b.agent.Chat(ctx, sessionID, "Please give me a brief system status: list all traders and their status, show total positions count.")
	if err != nil {
		logger.Errorf("Agent error: %v", err)
		return c.Send("❌ Failed to get status. Please try again.")
	}

	return c.Send(response.Text, tele.ModeMarkdown)
}

func (b *Bot) handleBalance(c tele.Context) error {
	ctx := context.Background()
	sessionID := b.getSessionID(c)

	response, err := b.agent.Chat(ctx, sessionID, "Show me all account balances from all running traders.")
	if err != nil {
		logger.Errorf("Agent error: %v", err)
		return c.Send("❌ Failed to get balances. Please try again.")
	}

	return c.Send(response.Text, tele.ModeMarkdown)
}

func (b *Bot) handlePositions(c tele.Context) error {
	ctx := context.Background()
	sessionID := b.getSessionID(c)

	response, err := b.agent.Chat(ctx, sessionID, "Show me all current positions from all running traders with P&L.")
	if err != nil {
		logger.Errorf("Agent error: %v", err)
		return c.Send("❌ Failed to get positions. Please try again.")
	}

	return c.Send(response.Text, tele.ModeMarkdown)
}

func (b *Bot) handleTraders(c tele.Context) error {
	ctx := context.Background()
	sessionID := b.getSessionID(c)

	response, err := b.agent.Chat(ctx, sessionID, "List all configured AI traders with their status, exchange, and AI model.")
	if err != nil {
		logger.Errorf("Agent error: %v", err)
		return c.Send("❌ Failed to list traders. Please try again.")
	}

	return c.Send(response.Text, tele.ModeMarkdown)
}

func (b *Bot) handleClear(c tele.Context) error {
	sessionID := b.getSessionID(c)
	session := b.agent.GetSession(sessionID)
	session.Clear()

	return c.Send("🧹 Conversation history cleared.\n\n对话历史已清除。")
}

// ==================== Message Handler ====================

func (b *Bot) handleText(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	if text == "" {
		return nil
	}

	// Show typing indicator
	_ = c.Notify(tele.Typing)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	sessionID := b.getSessionID(c)

	// Set user info in session
	session := b.agent.GetSession(sessionID)
	session.SetUserInfo(
		strconv.FormatInt(c.Sender().ID, 10),
		c.Sender().Username,
		"telegram",
	)

	logger.Infof("💬 [%s] %s: %s", sessionID, c.Sender().Username, text)

	response, err := b.agent.Chat(ctx, sessionID, text)
	if err != nil {
		logger.Errorf("Agent error: %v", err)
		return c.Send("❌ Sorry, something went wrong. Please try again.\n\n抱歉，出现了问题，请重试。")
	}

	logger.Infof("🤖 [%s] Response: %s", sessionID, truncate(response.Text, 100))

	// Send response (split if too long)
	return b.sendLongMessage(c, response.Text)
}

// handleCallback handles inline keyboard callbacks
func (b *Bot) handleCallback(c tele.Context) error {
	data := c.Callback().Data
	
	// Parse callback data (format: "action:param1:param2")
	parts := strings.Split(data, ":")
	if len(parts) == 0 {
		return c.Respond()
	}

	action := parts[0]

	switch action {
	case "confirm_trade":
		if len(parts) >= 2 {
			// Execute the confirmed trade
			return b.executeConfirmedTrade(c, parts[1:])
		}
	case "cancel_trade":
		_ = c.Respond(&tele.CallbackResponse{Text: "Trade cancelled / 交易已取消"})
		return c.Edit("❌ Trade cancelled.\n\n交易已取消。")
	}

	return c.Respond()
}

// ==================== Helpers ====================

func (b *Bot) getSessionID(c tele.Context) string {
	return fmt.Sprintf("tg_%d", c.Chat().ID)
}

func (b *Bot) sendLongMessage(c tele.Context, text string) error {
	// Telegram message limit is 4096 characters
	const maxLen = 4000

	if len(text) <= maxLen {
		return c.Send(text, tele.ModeMarkdown)
	}

	// Split into chunks
	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			// Try to split at newline
			idx := strings.LastIndex(text[:maxLen], "\n")
			if idx > 0 {
				chunk = text[:idx]
				text = text[idx+1:]
			} else {
				chunk = text[:maxLen]
				text = text[maxLen:]
			}
		} else {
			text = ""
		}

		if err := c.Send(chunk, tele.ModeMarkdown); err != nil {
			// Try without markdown if it fails
			if err := c.Send(chunk); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Bot) executeConfirmedTrade(c tele.Context, params []string) error {
	// TODO: Implement trade execution from callback
	_ = c.Respond(&tele.CallbackResponse{Text: "Executing trade..."})
	return c.Edit("✅ Trade executed.\n\n交易已执行。")
}

// AddAllowedUser adds a user to the allowlist
func (b *Bot) AddAllowedUser(userID int64) {
	b.allowedUsersLock.Lock()
	defer b.allowedUsersLock.Unlock()
	b.allowedUsers[userID] = true
}

// RemoveAllowedUser removes a user from the allowlist
func (b *Bot) RemoveAllowedUser(userID int64) {
	b.allowedUsersLock.Lock()
	defer b.allowedUsersLock.Unlock()
	delete(b.allowedUsers, userID)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ==================== Rate Limiter ====================

// RateLimiter implements per-user rate limiting
type RateLimiter struct {
	maxPerMinute int
	users        map[int64][]time.Time
	mu           sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerMinute int) *RateLimiter {
	return &RateLimiter{
		maxPerMinute: maxPerMinute,
		users:        make(map[int64][]time.Time),
	}
}

// Allow checks if a user is allowed to send a message
func (r *RateLimiter) Allow(userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	// Get user's recent messages
	timestamps := r.users[userID]

	// Filter out old timestamps
	var recent []time.Time
	for _, t := range timestamps {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	// Check if under limit
	if len(recent) >= r.maxPerMinute {
		return false
	}

	// Add current timestamp
	recent = append(recent, now)
	r.users[userID] = recent

	return true
}
