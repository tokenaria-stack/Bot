package ui_config

import "trading_bot/core"

// BuildUIRegistry assembles the DDR UI registry explicitly (no init() globals).
func BuildUIRegistry() (*core.UIRegistry, error) {
	r := core.NewUIRegistry()
	if err := r.Register(RSXComponents()...); err != nil {
		return nil, err
	}
	if err := r.Register(WozduhComponents()...); err != nil {
		return nil, err
	}
	return r, nil
}
