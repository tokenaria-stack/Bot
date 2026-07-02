package strategy

import "testing"

func TestUpdateRiskSettings_NilInstanceSafe(t *testing.T) {
	riskSettingsMutex.Lock()
	saved := riskSettingsInstance
	riskSettingsInstance = nil
	riskSettingsMutex.Unlock()
	t.Cleanup(func() {
		riskSettingsMutex.Lock()
		if riskSettingsInstance == nil && saved != nil {
			riskSettingsInstance = saved
		}
		riskSettingsMutex.Unlock()
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UpdateRiskSettings panicked: %v", r)
		}
	}()

	UpdateRiskSettings(RiskSettings{RiskPerTrade: 2.0, Leverage: 8})

	got := GetRiskSettings()
	if got == nil {
		t.Fatal("GetRiskSettings returned nil")
	}
	if got.RiskPerTrade != 2.0 {
		t.Fatalf("RiskPerTrade = %v, want 2.0", got.RiskPerTrade)
	}
	if got.Leverage != 8 {
		t.Fatalf("Leverage = %v, want 8", got.Leverage)
	}
}
