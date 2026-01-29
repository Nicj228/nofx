package assistant

import (
	"fmt"
	"nofx/logger"
	"nofx/manager"
	"nofx/store"
	"sync"
	"time"
)

// Monitor provides proactive monitoring and alerts
type Monitor struct {
	traderManager  *manager.TraderManager
	store          *store.Store
	contextBuilder *ContextBuilder
	
	// Alert callbacks
	alertCallbacks []func(Alert)
	callbackMu     sync.RWMutex
	
	// State
	running    bool
	stopChan   chan struct{}
	interval   time.Duration
	
	// Last known state for change detection
	lastPositions map[string]PositionSummary
	lastAlerts    map[string]time.Time // Prevent alert spam
	mu            sync.RWMutex
}

// NewMonitor creates a new trading monitor
func NewMonitor(tm *manager.TraderManager, st *store.Store) *Monitor {
	return &Monitor{
		traderManager:  tm,
		store:          st,
		contextBuilder: NewContextBuilder(tm, st),
		stopChan:       make(chan struct{}),
		interval:       30 * time.Second, // Check every 30 seconds
		lastPositions:  make(map[string]PositionSummary),
		lastAlerts:     make(map[string]time.Time),
	}
}

// OnAlert registers an alert callback
func (m *Monitor) OnAlert(callback func(Alert)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.alertCallbacks = append(m.alertCallbacks, callback)
}

// Start starts the monitor
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()
	
	logger.Info("🔍 Starting trading monitor...")
	
	go m.monitorLoop()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return
	}
	
	m.running = false
	close(m.stopChan)
	logger.Info("🔍 Trading monitor stopped")
}

// monitorLoop is the main monitoring loop
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	
	// Initial check
	m.checkAndAlert()
	
	for {
		select {
		case <-ticker.C:
			m.checkAndAlert()
		case <-m.stopChan:
			return
		}
	}
}

// checkAndAlert checks positions and sends alerts
func (m *Monitor) checkAndAlert() {
	ctx := m.contextBuilder.BuildContext()
	
	// Process built-in alerts from context
	for _, alert := range ctx.Alerts {
		m.sendAlertIfNew(alert)
	}
	
	// Check for position changes
	m.checkPositionChanges(ctx)
	
	// Check for new large movements
	m.checkMarketMovements(ctx)
}

// checkPositionChanges detects significant position changes
func (m *Monitor) checkPositionChanges(ctx *TradingContext) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	currentPositions := make(map[string]PositionSummary)
	
	for _, pos := range ctx.Positions {
		key := fmt.Sprintf("%s_%s_%s", pos.TraderID, pos.Symbol, pos.Side)
		currentPositions[key] = pos
		
		// Check if this is a new position
		if _, existed := m.lastPositions[key]; !existed {
			m.sendAlert(Alert{
				Level:   "info",
				Type:    "new_position",
				Message: fmt.Sprintf("📍 新开仓位: %s %s %.4f @ %.2f (%dx)", 
					pos.Symbol, pos.Side, pos.Size, pos.EntryPrice, pos.Leverage),
			})
		}
	}
	
	// Check for closed positions
	for key, oldPos := range m.lastPositions {
		if _, exists := currentPositions[key]; !exists {
			m.sendAlert(Alert{
				Level:   "info",
				Type:    "position_closed",
				Message: fmt.Sprintf("📍 仓位已平: %s %s (入场价: %.2f)", 
					oldPos.Symbol, oldPos.Side, oldPos.EntryPrice),
			})
		}
	}
	
	m.lastPositions = currentPositions
}

// checkMarketMovements checks for significant market movements
func (m *Monitor) checkMarketMovements(ctx *TradingContext) {
	// This could be expanded to check price movements
	// For now, we rely on the context builder's alerts
}

// sendAlertIfNew sends an alert only if it's new (avoid spam)
func (m *Monitor) sendAlertIfNew(alert Alert) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := fmt.Sprintf("%s_%s", alert.Type, alert.Message)
	
	// Check if we sent this alert recently (within 5 minutes)
	if lastSent, ok := m.lastAlerts[key]; ok {
		if time.Since(lastSent) < 5*time.Minute {
			return // Skip, already sent recently
		}
	}
	
	m.lastAlerts[key] = time.Now()
	m.sendAlert(alert)
}

// sendAlert sends alert to all registered callbacks
func (m *Monitor) sendAlert(alert Alert) {
	m.callbackMu.RLock()
	callbacks := make([]func(Alert), len(m.alertCallbacks))
	copy(callbacks, m.alertCallbacks)
	m.callbackMu.RUnlock()
	
	for _, cb := range callbacks {
		go cb(alert)
	}
}

// GetCurrentContext returns the current trading context
func (m *Monitor) GetCurrentContext() *TradingContext {
	return m.contextBuilder.BuildContext()
}

// SetInterval sets the monitoring interval
func (m *Monitor) SetInterval(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interval = d
}
