package strategy

import "sync"

// RiskSettings хранит глобальные параметры управления капиталом
type RiskSettings struct {
	RiskPerTrade  float64 `json:"risk_per_trade"` // Процент от депозита (напр., 1.0)
	MaxDrawdown   float64 `json:"max_drawdown"`   // Максимальная просадка за день
	Leverage      int     `json:"leverage"`       // Плечо
	StopLossType  string  `json:"stop_loss_type"` // "fractal_atr", "fixed_pct"
	ATRMultiplier float64 `json:"atr_multiplier"` // Множитель для ATR (напр., 1.5)
}

var (
	riskSettingsInstance *RiskSettings
	riskSettingsOnce     sync.Once
	riskSettingsMutex    sync.RWMutex
)

func GetRiskSettings() *RiskSettings {
	riskSettingsOnce.Do(func() {
		riskSettingsInstance = defaultRiskSettings()
	})

	riskSettingsMutex.RLock()
	defer riskSettingsMutex.RUnlock()

	copy := *riskSettingsInstance
	return &copy
}

func defaultRiskSettings() *RiskSettings {
	return &RiskSettings{
		RiskPerTrade:  1.0,
		MaxDrawdown:   5.0,
		Leverage:      10,
		StopLossType:  "fractal_atr",
		ATRMultiplier: 1.5,
	}
}

func ensureRiskSettings() {
	riskSettingsOnce.Do(func() {
		riskSettingsInstance = defaultRiskSettings()
	})
}

func UpdateRiskSettings(newSettings RiskSettings) {
	ensureRiskSettings()

	riskSettingsMutex.Lock()
	defer riskSettingsMutex.Unlock()

	if riskSettingsInstance == nil {
		riskSettingsInstance = defaultRiskSettings()
	}

	riskSettingsInstance.RiskPerTrade = newSettings.RiskPerTrade
	riskSettingsInstance.MaxDrawdown = newSettings.MaxDrawdown
	riskSettingsInstance.Leverage = newSettings.Leverage

	if newSettings.StopLossType != "" {
		riskSettingsInstance.StopLossType = newSettings.StopLossType
	}
	if newSettings.ATRMultiplier > 0 {
		riskSettingsInstance.ATRMultiplier = newSettings.ATRMultiplier
	}
}
