// Package assistant - Trading Context Builder
// Automatically enriches AI prompts with real-time market and portfolio data
package assistant

import (
	"fmt"
	"nofx/manager"
	"nofx/store"
	"strings"
	"time"
)

// TradingContext holds real-time trading context for AI decision making
type TradingContext struct {
	// Portfolio state
	TotalEquity      float64                  `json:"total_equity"`
	AvailableBalance float64                  `json:"available_balance"`
	UnrealizedPnL    float64                  `json:"unrealized_pnl"`
	Positions        []PositionSummary        `json:"positions"`
	
	// Market data
	MarketPrices     map[string]float64       `json:"market_prices"`
	PriceChanges24h  map[string]float64       `json:"price_changes_24h"`
	
	// Trader states
	ActiveTraders    []TraderSummary          `json:"active_traders"`
	
	// Alerts
	Alerts           []Alert                  `json:"alerts"`
	
	// Timestamp
	UpdatedAt        time.Time                `json:"updated_at"`
}

// PositionSummary summarizes a position
type PositionSummary struct {
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"` // "long" or "short"
	Size          float64 `json:"size"`
	EntryPrice    float64 `json:"entry_price"`
	MarkPrice     float64 `json:"mark_price"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	PnLPercent    float64 `json:"pnl_percent"`
	Leverage      int     `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price,omitempty"`
	TraderID      string  `json:"trader_id"`
	TraderName    string  `json:"trader_name"`
}

// TraderSummary summarizes a trader's state
type TraderSummary struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Exchange      string  `json:"exchange"`
	IsRunning     bool    `json:"is_running"`
	Equity        float64 `json:"equity"`
	PositionCount int     `json:"position_count"`
	TodayPnL      float64 `json:"today_pnl,omitempty"`
}

// Alert represents a trading alert
type Alert struct {
	Level   string `json:"level"` // "info", "warning", "danger"
	Type    string `json:"type"`  // "liquidation_risk", "large_loss", "price_alert", etc.
	Message string `json:"message"`
}

// ContextBuilder builds trading context for AI
type ContextBuilder struct {
	traderManager *manager.TraderManager
	store         *store.Store
}

// NewContextBuilder creates a context builder
func NewContextBuilder(tm *manager.TraderManager, st *store.Store) *ContextBuilder {
	return &ContextBuilder{
		traderManager: tm,
		store:         st,
	}
}

// BuildContext builds current trading context
func (cb *ContextBuilder) BuildContext() *TradingContext {
	ctx := &TradingContext{
		MarketPrices:    make(map[string]float64),
		PriceChanges24h: make(map[string]float64),
		UpdatedAt:       time.Now(),
	}

	// Get all traders
	allTraders := cb.traderManager.GetAllTraders()
	
	for id, trader := range allTraders {
		summary := TraderSummary{
			ID:        id,
			Name:      trader.GetName(),
			Exchange:  trader.GetExchange(),
			IsRunning: true, // If in map, it's running
		}

		// Get account info
		if accountInfo, err := trader.GetAccountInfo(); err == nil {
			if equity, ok := accountInfo["total_equity"].(float64); ok {
				summary.Equity = equity
				ctx.TotalEquity += equity
			}
			if available, ok := accountInfo["available_balance"].(float64); ok {
				ctx.AvailableBalance += available
			}
		}

		// Get positions
		if positions, err := trader.GetPositions(); err == nil {
			summary.PositionCount = len(positions)
			
			for _, pos := range positions {
				posSummary := cb.parsePosition(pos, id, trader.GetName())
				if posSummary != nil {
					ctx.Positions = append(ctx.Positions, *posSummary)
					ctx.UnrealizedPnL += posSummary.UnrealizedPnL
					
					// Track market prices
					ctx.MarketPrices[posSummary.Symbol] = posSummary.MarkPrice
					
					// Check for alerts
					cb.checkPositionAlerts(ctx, posSummary)
				}
			}
		}

		ctx.ActiveTraders = append(ctx.ActiveTraders, summary)
	}

	return ctx
}

// parsePosition parses position data into summary
func (cb *ContextBuilder) parsePosition(pos map[string]interface{}, traderID, traderName string) *PositionSummary {
	summary := &PositionSummary{
		TraderID:   traderID,
		TraderName: traderName,
	}

	if symbol, ok := pos["symbol"].(string); ok {
		summary.Symbol = symbol
	}
	if side, ok := pos["side"].(string); ok {
		summary.Side = strings.ToLower(side)
	}
	if size, ok := pos["size"].(float64); ok {
		summary.Size = size
	}
	if entry, ok := pos["entry_price"].(float64); ok {
		summary.EntryPrice = entry
	}
	if mark, ok := pos["mark_price"].(float64); ok {
		summary.MarkPrice = mark
	}
	if pnl, ok := pos["unrealized_pnl"].(float64); ok {
		summary.UnrealizedPnL = pnl
	}
	if lev, ok := pos["leverage"].(int); ok {
		summary.Leverage = lev
	}
	if liq, ok := pos["liquidation_price"].(float64); ok {
		summary.LiquidationPrice = liq
	}

	// Calculate PnL percent
	if summary.EntryPrice > 0 && summary.Size > 0 {
		if summary.Side == "long" {
			summary.PnLPercent = ((summary.MarkPrice - summary.EntryPrice) / summary.EntryPrice) * 100 * float64(summary.Leverage)
		} else {
			summary.PnLPercent = ((summary.EntryPrice - summary.MarkPrice) / summary.EntryPrice) * 100 * float64(summary.Leverage)
		}
	}

	return summary
}

// checkPositionAlerts checks for position-related alerts
func (cb *ContextBuilder) checkPositionAlerts(ctx *TradingContext, pos *PositionSummary) {
	// Liquidation risk alert
	if pos.LiquidationPrice > 0 && pos.MarkPrice > 0 {
		var distancePercent float64
		if pos.Side == "long" {
			distancePercent = ((pos.MarkPrice - pos.LiquidationPrice) / pos.MarkPrice) * 100
		} else {
			distancePercent = ((pos.LiquidationPrice - pos.MarkPrice) / pos.MarkPrice) * 100
		}

		if distancePercent < 5 {
			ctx.Alerts = append(ctx.Alerts, Alert{
				Level:   "danger",
				Type:    "liquidation_risk",
				Message: fmt.Sprintf("⚠️ %s %s仓位距离强平仅 %.1f%%！", pos.Symbol, pos.Side, distancePercent),
			})
		} else if distancePercent < 10 {
			ctx.Alerts = append(ctx.Alerts, Alert{
				Level:   "warning",
				Type:    "liquidation_risk",
				Message: fmt.Sprintf("⚡ %s %s仓位距离强平 %.1f%%，注意风险", pos.Symbol, pos.Side, distancePercent),
			})
		}
	}

	// Large loss alert
	if pos.PnLPercent < -20 {
		ctx.Alerts = append(ctx.Alerts, Alert{
			Level:   "danger",
			Type:    "large_loss",
			Message: fmt.Sprintf("📉 %s %s仓位亏损 %.1f%%，考虑止损", pos.Symbol, pos.Side, pos.PnLPercent),
		})
	} else if pos.PnLPercent < -10 {
		ctx.Alerts = append(ctx.Alerts, Alert{
			Level:   "warning",
			Type:    "large_loss",
			Message: fmt.Sprintf("📉 %s %s仓位亏损 %.1f%%", pos.Symbol, pos.Side, pos.PnLPercent),
		})
	}

	// Large profit - consider taking profit
	if pos.PnLPercent > 50 {
		ctx.Alerts = append(ctx.Alerts, Alert{
			Level:   "info",
			Type:    "large_profit",
			Message: fmt.Sprintf("📈 %s %s仓位盈利 %.1f%%，考虑部分止盈", pos.Symbol, pos.Side, pos.PnLPercent),
		})
	}
}

// FormatContextForPrompt formats context as text for AI prompt injection
func (ctx *TradingContext) FormatContextForPrompt() string {
	var sb strings.Builder

	sb.WriteString("\n\n---\n## 📊 当前交易状态 (实时)\n\n")

	// Portfolio summary
	sb.WriteString(fmt.Sprintf("**总权益:** $%.2f | **可用余额:** $%.2f | **未实现盈亏:** $%.2f\n\n",
		ctx.TotalEquity, ctx.AvailableBalance, ctx.UnrealizedPnL))

	// Alerts (high priority)
	if len(ctx.Alerts) > 0 {
		sb.WriteString("### ⚠️ 警报\n")
		for _, alert := range ctx.Alerts {
			sb.WriteString(fmt.Sprintf("- %s\n", alert.Message))
		}
		sb.WriteString("\n")
	}

	// Active positions
	if len(ctx.Positions) > 0 {
		sb.WriteString("### 📈 持仓\n")
		sb.WriteString("| 交易对 | 方向 | 数量 | 入场价 | 现价 | 盈亏 | 盈亏% | 杠杆 | 交易员 |\n")
		sb.WriteString("|--------|------|------|--------|------|------|-------|------|--------|\n")
		for _, pos := range ctx.Positions {
			pnlEmoji := "🟢"
			if pos.UnrealizedPnL < 0 {
				pnlEmoji = "🔴"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %.4f | %.2f | %.2f | %s$%.2f | %.1f%% | %dx | %s |\n",
				pos.Symbol, pos.Side, pos.Size, pos.EntryPrice, pos.MarkPrice,
				pnlEmoji, pos.UnrealizedPnL, pos.PnLPercent, pos.Leverage, pos.TraderName))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("### 📈 持仓\n无持仓\n\n")
	}

	// Active traders
	if len(ctx.ActiveTraders) > 0 {
		sb.WriteString("### 🤖 运行中的交易员\n")
		for _, t := range ctx.ActiveTraders {
			status := "✅ 运行中"
			if !t.IsRunning {
				status = "❌ 已停止"
			}
			sb.WriteString(fmt.Sprintf("- **%s** (%s) %s | 权益: $%.2f | 持仓: %d\n",
				t.Name, t.Exchange, status, t.Equity, t.PositionCount))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("*数据更新时间: %s*\n---\n", ctx.UpdatedAt.Format("2006-01-02 15:04:05")))

	return sb.String()
}

// GetTopSymbols returns symbols with positions for market data queries
func (ctx *TradingContext) GetTopSymbols() []string {
	symbolSet := make(map[string]bool)
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}
	
	// Always include major pairs
	symbolSet["BTCUSDT"] = true
	symbolSet["ETHUSDT"] = true
	
	symbols := make([]string, 0, len(symbolSet))
	for s := range symbolSet {
		symbols = append(symbols, s)
	}
	return symbols
}

// EnrichWithMarketData adds market data to context
// Note: Market prices are already populated from position data
func (cb *ContextBuilder) EnrichWithMarketData(ctx *TradingContext, symbols []string) {
	// Market prices are populated from position mark prices
	// Additional market data enrichment can be added here in the future
}
