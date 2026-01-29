// Package assistant - Intelligent Strategy Builder
// Allows users to create powerful, flexible trading strategies through natural language
package assistant

import (
	"fmt"
	"nofx/store"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StrategyType defines the type of trading strategy
type StrategyType string

const (
	StrategyTypeAI           StrategyType = "ai"            // AI decides everything
	StrategyTypeTrend        StrategyType = "trend"         // Trend following
	StrategyTypeMeanRevert   StrategyType = "mean_revert"   // Mean reversion
	StrategyTypeGrid         StrategyType = "grid"          // Grid trading
	StrategyTypeDCA          StrategyType = "dca"           // Dollar cost averaging
	StrategyTypeBreakout     StrategyType = "breakout"      // Breakout trading
	StrategyTypeArbitrage    StrategyType = "arbitrage"     // Cross-exchange arbitrage
	StrategyTypeCustom       StrategyType = "custom"        // Custom rules
)

// SmartStrategy represents a user-defined trading strategy
type SmartStrategy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        StrategyType `json:"type"`
	
	// Trading pairs
	Symbols     []string     `json:"symbols"`      // e.g., ["BTCUSDT", "ETHUSDT"]
	SymbolMode  string       `json:"symbol_mode"`  // "static", "ai_select", "top_volume", "top_oi"
	MaxSymbols  int          `json:"max_symbols"`  // Max symbols to trade simultaneously
	
	// Entry conditions
	EntryRules  []Rule       `json:"entry_rules"`
	EntryMode   string       `json:"entry_mode"`   // "any" (OR) or "all" (AND)
	
	// Exit conditions
	ExitRules   []Rule       `json:"exit_rules"`
	TakeProfit  *float64     `json:"take_profit"`  // TP percentage
	StopLoss    *float64     `json:"stop_loss"`    // SL percentage
	TrailingStop *float64    `json:"trailing_stop"` // Trailing stop percentage
	
	// Position sizing
	PositionSize    PositionSizeConfig `json:"position_size"`
	MaxPositions    int                `json:"max_positions"`     // Max concurrent positions
	MaxPerSymbol    int                `json:"max_per_symbol"`    // Max positions per symbol
	
	// Risk management
	RiskConfig      RiskConfig         `json:"risk_config"`
	
	// Leverage settings
	LeverageConfig  LeverageConfig     `json:"leverage_config"`
	
	// Time settings
	TimeConfig      TimeConfig         `json:"time_config"`
	
	// AI enhancement
	AIConfig        AIStrategyConfig   `json:"ai_config"`
	
	// Metadata
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
	IsActive    bool       `json:"is_active"`
	Performance *StrategyPerformance `json:"performance,omitempty"`
}

// Rule represents a trading rule/condition
type Rule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`        // "indicator", "price", "time", "volume", "ai", "custom"
	Indicator   string      `json:"indicator"`   // e.g., "RSI", "MACD", "EMA"
	Condition   string      `json:"condition"`   // e.g., "crosses_above", "greater_than", "less_than"
	Value       interface{} `json:"value"`       // The value to compare against
	Timeframe   string      `json:"timeframe"`   // e.g., "1h", "4h", "1d"
	Weight      float64     `json:"weight"`      // Weight for scoring (0-1)
	Description string      `json:"description"` // Human readable description
}

// PositionSizeConfig defines how to size positions
type PositionSizeConfig struct {
	Mode            string  `json:"mode"`             // "fixed", "percent", "risk_based", "kelly"
	FixedAmount     float64 `json:"fixed_amount"`     // Fixed USDT amount
	PercentOfEquity float64 `json:"percent_of_equity"` // Percentage of total equity
	RiskPerTrade    float64 `json:"risk_per_trade"`   // Max risk per trade (%)
	MaxSingleTrade  float64 `json:"max_single_trade"` // Max single trade size (USDT)
}

// RiskConfig defines risk management rules
type RiskConfig struct {
	MaxDrawdown        float64 `json:"max_drawdown"`         // Max drawdown before stopping (%)
	MaxDailyLoss       float64 `json:"max_daily_loss"`       // Max daily loss (%)
	MaxOpenRisk        float64 `json:"max_open_risk"`        // Max total open risk (%)
	CooldownAfterLoss  int     `json:"cooldown_after_loss"`  // Minutes to wait after a loss
	RequireConfirmation bool   `json:"require_confirmation"` // Require user confirmation for trades
	EmergencyStopLoss  float64 `json:"emergency_stop_loss"`  // Emergency SL for all positions (%)
}

// LeverageConfig defines leverage settings
type LeverageConfig struct {
	Mode           string             `json:"mode"`            // "fixed", "dynamic", "per_symbol"
	DefaultLeverage int               `json:"default_leverage"`
	MaxLeverage    int                `json:"max_leverage"`
	PerSymbol      map[string]int     `json:"per_symbol"`      // Symbol-specific leverage
	PerVolatility  []VolatilityLever  `json:"per_volatility"`  // Volatility-based leverage
}

// VolatilityLever defines leverage based on volatility
type VolatilityLever struct {
	MaxVolatility float64 `json:"max_volatility"` // ATR percentage threshold
	Leverage      int     `json:"leverage"`
}

// TimeConfig defines time-based settings
type TimeConfig struct {
	TradingHours    []TimeRange `json:"trading_hours"`    // When to trade
	AvoidNews       bool        `json:"avoid_news"`       // Avoid major news events
	AvoidWeekends   bool        `json:"avoid_weekends"`
	MinHoldTime     int         `json:"min_hold_time"`    // Minimum hold time (minutes)
	MaxHoldTime     int         `json:"max_hold_time"`    // Maximum hold time (minutes)
	ScanInterval    int         `json:"scan_interval"`    // How often to scan (minutes)
}

// TimeRange represents a time range
type TimeRange struct {
	Start string `json:"start"` // "09:00"
	End   string `json:"end"`   // "17:00"
	TZ    string `json:"tz"`    // Timezone
}

// AIStrategyConfig defines AI-specific settings
type AIStrategyConfig struct {
	Enabled           bool     `json:"enabled"`
	Model             string   `json:"model"`              // AI model to use
	ConfidenceThreshold float64 `json:"confidence_threshold"` // Min confidence to act
	UseMarketSentiment bool    `json:"use_market_sentiment"`
	UseTechnicalAnalysis bool  `json:"use_technical_analysis"`
	UseOnChainData    bool     `json:"use_onchain_data"`
	CustomPrompt      string   `json:"custom_prompt"`       // Custom instructions for AI
	Personality       string   `json:"personality"`         // "aggressive", "conservative", "balanced"
}

// StrategyPerformance tracks strategy performance
type StrategyPerformance struct {
	TotalTrades     int       `json:"total_trades"`
	WinningTrades   int       `json:"winning_trades"`
	LosingTrades    int       `json:"losing_trades"`
	WinRate         float64   `json:"win_rate"`
	TotalPnL        float64   `json:"total_pnl"`
	MaxDrawdown     float64   `json:"max_drawdown"`
	SharpeRatio     float64   `json:"sharpe_ratio"`
	ProfitFactor    float64   `json:"profit_factor"`
	AvgWin          float64   `json:"avg_win"`
	AvgLoss         float64   `json:"avg_loss"`
	LastUpdated     time.Time `json:"last_updated"`
}

// StrategyBuilder helps users create strategies through conversation
type StrategyBuilder struct {
	store *store.Store
}

// NewStrategyBuilder creates a new strategy builder
func NewStrategyBuilder(st *store.Store) *StrategyBuilder {
	return &StrategyBuilder{store: st}
}

// CreateFromNaturalLanguage creates a strategy from natural language description
func (sb *StrategyBuilder) CreateFromNaturalLanguage(description string, userID string) (*SmartStrategy, error) {
	// This would typically call an AI to parse the description
	// For now, we create a basic template
	strategy := &SmartStrategy{
		ID:          uuid.New().String()[:8],
		Name:        "Custom Strategy",
		Description: description,
		Type:        StrategyTypeAI,
		SymbolMode:  "ai_select",
		MaxSymbols:  5,
		EntryMode:   "all",
		MaxPositions: 5,
		MaxPerSymbol: 1,
		PositionSize: PositionSizeConfig{
			Mode:            "percent",
			PercentOfEquity: 5,
			MaxSingleTrade:  1000,
		},
		RiskConfig: RiskConfig{
			MaxDrawdown:        20,
			MaxDailyLoss:       5,
			MaxOpenRisk:        10,
			CooldownAfterLoss:  30,
			RequireConfirmation: true,
			EmergencyStopLoss:  30,
		},
		LeverageConfig: LeverageConfig{
			Mode:            "dynamic",
			DefaultLeverage: 3,
			MaxLeverage:     10,
		},
		TimeConfig: TimeConfig{
			ScanInterval:  5,
			AvoidWeekends: false,
		},
		AIConfig: AIStrategyConfig{
			Enabled:             true,
			ConfidenceThreshold: 0.7,
			UseMarketSentiment:  true,
			UseTechnicalAnalysis: true,
			Personality:         "balanced",
			CustomPrompt:        description,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: userID,
		IsActive:  false,
	}

	return strategy, nil
}

// CreateGridStrategy creates a grid trading strategy
func (sb *StrategyBuilder) CreateGridStrategy(symbol string, lowerPrice, upperPrice float64, gridCount int, amountPerGrid float64) *SmartStrategy {
	return &SmartStrategy{
		ID:          uuid.New().String()[:8],
		Name:        fmt.Sprintf("Grid %s", symbol),
		Description: fmt.Sprintf("Grid trading %s from %.2f to %.2f with %d grids", symbol, lowerPrice, upperPrice, gridCount),
		Type:        StrategyTypeGrid,
		Symbols:     []string{symbol},
		SymbolMode:  "static",
		MaxPositions: gridCount,
		PositionSize: PositionSizeConfig{
			Mode:        "fixed",
			FixedAmount: amountPerGrid,
		},
		EntryRules: []Rule{
			{
				ID:        "grid_entry",
				Type:      "price",
				Condition: "grid_level",
				Value: map[string]interface{}{
					"lower_price": lowerPrice,
					"upper_price": upperPrice,
					"grid_count":  gridCount,
				},
			},
		},
		CreatedAt: time.Now(),
		IsActive:  false,
	}
}

// CreateDCAStrategy creates a DCA strategy
func (sb *StrategyBuilder) CreateDCAStrategy(symbol string, intervalMinutes int, amountPerBuy float64, maxBuys int) *SmartStrategy {
	return &SmartStrategy{
		ID:          uuid.New().String()[:8],
		Name:        fmt.Sprintf("DCA %s", symbol),
		Description: fmt.Sprintf("DCA into %s every %d minutes, $%.2f per buy, max %d buys", symbol, intervalMinutes, amountPerBuy, maxBuys),
		Type:        StrategyTypeDCA,
		Symbols:     []string{symbol},
		SymbolMode:  "static",
		MaxPositions: maxBuys,
		PositionSize: PositionSizeConfig{
			Mode:        "fixed",
			FixedAmount: amountPerBuy,
		},
		TimeConfig: TimeConfig{
			ScanInterval: intervalMinutes,
		},
		CreatedAt: time.Now(),
		IsActive:  false,
	}
}

// CreateTrendStrategy creates a trend following strategy
func (sb *StrategyBuilder) CreateTrendStrategy(symbols []string, emaFast, emaSlow int, leverage int) *SmartStrategy {
	return &SmartStrategy{
		ID:          uuid.New().String()[:8],
		Name:        "Trend Following",
		Description: fmt.Sprintf("EMA %d/%d crossover strategy", emaFast, emaSlow),
		Type:        StrategyTypeTrend,
		Symbols:     symbols,
		SymbolMode:  "static",
		EntryMode:   "all",
		EntryRules: []Rule{
			{
				ID:        "ema_cross",
				Name:      "EMA Crossover",
				Type:      "indicator",
				Indicator: "EMA",
				Condition: "crosses_above",
				Value: map[string]int{
					"fast_period": emaFast,
					"slow_period": emaSlow,
				},
				Timeframe: "1h",
				Weight:    1.0,
			},
		},
		ExitRules: []Rule{
			{
				ID:        "ema_cross_exit",
				Name:      "EMA Crossover Exit",
				Type:      "indicator",
				Indicator: "EMA",
				Condition: "crosses_below",
				Value: map[string]int{
					"fast_period": emaFast,
					"slow_period": emaSlow,
				},
				Timeframe: "1h",
			},
		},
		LeverageConfig: LeverageConfig{
			Mode:            "fixed",
			DefaultLeverage: leverage,
		},
		CreatedAt: time.Now(),
		IsActive:  false,
	}
}

// StrategyToPrompt converts a strategy to an AI prompt
func StrategyToPrompt(s *SmartStrategy) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# 策略: %s\n\n", s.Name))
	sb.WriteString(fmt.Sprintf("**描述**: %s\n", s.Description))
	sb.WriteString(fmt.Sprintf("**类型**: %s\n\n", s.Type))

	// Trading pairs
	if len(s.Symbols) > 0 {
		sb.WriteString(fmt.Sprintf("**交易对**: %s\n", strings.Join(s.Symbols, ", ")))
	} else {
		sb.WriteString(fmt.Sprintf("**选币模式**: %s (最多 %d 个)\n", s.SymbolMode, s.MaxSymbols))
	}

	// Entry rules
	if len(s.EntryRules) > 0 {
		sb.WriteString("\n## 入场规则\n")
		for _, rule := range s.EntryRules {
			sb.WriteString(fmt.Sprintf("- %s: %s %s %v\n", rule.Name, rule.Indicator, rule.Condition, rule.Value))
		}
	}

	// Exit rules
	sb.WriteString("\n## 出场规则\n")
	if s.TakeProfit != nil {
		sb.WriteString(fmt.Sprintf("- 止盈: %.1f%%\n", *s.TakeProfit))
	}
	if s.StopLoss != nil {
		sb.WriteString(fmt.Sprintf("- 止损: %.1f%%\n", *s.StopLoss))
	}
	if s.TrailingStop != nil {
		sb.WriteString(fmt.Sprintf("- 移动止损: %.1f%%\n", *s.TrailingStop))
	}

	// Risk management
	sb.WriteString("\n## 风险管理\n")
	sb.WriteString(fmt.Sprintf("- 最大回撤: %.1f%%\n", s.RiskConfig.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("- 单日最大亏损: %.1f%%\n", s.RiskConfig.MaxDailyLoss))
	sb.WriteString(fmt.Sprintf("- 最大持仓数: %d\n", s.MaxPositions))

	// AI settings
	if s.AIConfig.Enabled {
		sb.WriteString("\n## AI 配置\n")
		sb.WriteString(fmt.Sprintf("- 置信度阈值: %.0f%%\n", s.AIConfig.ConfidenceThreshold*100))
		sb.WriteString(fmt.Sprintf("- 风格: %s\n", s.AIConfig.Personality))
		if s.AIConfig.CustomPrompt != "" {
			sb.WriteString(fmt.Sprintf("- 自定义指令: %s\n", s.AIConfig.CustomPrompt))
		}
	}

	return sb.String()
}
