package starlens

import (
	"strings"

	"trading_bot/market"
)

// CatalogEntry is display metadata for one certified research coordinate.
// ID is the evaluator Field. Labels are not research identity.
type CatalogEntry struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	Section   string `json:"section"`
	Timeframe string `json:"timeframe"`
	Family    string `json:"family"`
	Unit      string `json:"unit"`
}

// ResearchCatalog lists the 191 Schema 3 and Matrix Field IDs.
// Names come from RawObservations and RelationNames.
func ResearchCatalog() []CatalogEntry {
	raw := market.RawObservations(market.StarSnapshotV3{})
	rels := market.RelationNames()
	out := make([]CatalogEntry, 0, len(raw)+len(rels))
	for _, o := range raw {
		out = append(out, schema3Catalog(o.Name))
	}
	for _, id := range rels {
		out = append(out, matrixCatalog(id))
	}
	return out
}

func schema3Catalog(id string) CatalogEntry {
	e := CatalogEntry{ID: id, Group: "Schema 3"}
	switch {
	case strings.HasPrefix(id, "H1RSX."):
		e.Timeframe, e.Section = "1h", "RSX"
		e.Family, e.Unit, e.Label = rsxFieldMeta(strings.TrimPrefix(id, "H1RSX."))
		e.Label = "1h " + e.Label
	case strings.HasPrefix(id, "H4RSX."):
		e.Timeframe, e.Section = "4h", "RSX"
		e.Family, e.Unit, e.Label = rsxFieldMeta(strings.TrimPrefix(id, "H4RSX."))
		e.Label = "4h " + e.Label
	case strings.HasPrefix(id, "D1RSX."):
		e.Timeframe, e.Section = "Daily", "RSX"
		e.Family, e.Unit, e.Label = rsxFieldMeta(strings.TrimPrefix(id, "D1RSX."))
		e.Label = "Daily " + e.Label
	case strings.HasPrefix(id, "M15."):
		e.Timeframe, e.Section = "15m", "15m"
		e.Family, e.Unit, e.Label = tfFieldMeta(strings.TrimPrefix(id, "M15."))
		e.Label = "15m " + e.Label
	case strings.HasPrefix(id, "H1."):
		e.Timeframe, e.Section = "1h", "1h"
		e.Family, e.Unit, e.Label = tfFieldMeta(strings.TrimPrefix(id, "H1."))
		e.Label = "1h " + e.Label
	case strings.HasPrefix(id, "H4."):
		e.Timeframe, e.Section = "4h", "4h"
		e.Family, e.Unit, e.Label = tfFieldMeta(strings.TrimPrefix(id, "H4."))
		e.Label = "4h " + e.Label
	case strings.HasPrefix(id, "D1."):
		e.Timeframe, e.Section = "Daily", "Daily"
		e.Family, e.Unit, e.Label = tfFieldMeta(strings.TrimPrefix(id, "D1."))
		e.Label = "Daily " + e.Label
	case id == "TVAge":
		e.Timeframe, e.Section, e.Family, e.Unit, e.Label = "15m", "TV", "TV", "15m bars", "TV age"
	default:
		e.Timeframe, e.Section = "15m", "RSX"
		e.Family, e.Unit, e.Label = rootRSXMeta(id)
	}
	return e
}

func matrixCatalog(id string) CatalogEntry {
	e := CatalogEntry{ID: id, Group: "Matrix", Label: id, Family: "relation", Unit: "stored gap"}
	switch {
	case strings.Contains(id, "RsxMinusSignal"):
		e.Section = "RSX − Signal"
		e.Timeframe = matrixRSXTF(id)
		e.Label = e.Timeframe + " RSX − signal"
	case strings.HasSuffix(id, "CloseWidth") || strings.HasSuffix(id, "Width"):
		e.Section = "Neighboring Widths"
		e.Timeframe, e.Label = neighborLabel(id)
	case strings.Contains(id, "H1H4") || strings.Contains(id, "M15H1") || strings.Contains(id, "H4D1"):
		e.Section = "Neighboring Levels"
		e.Timeframe, e.Label = neighborLabel(id)
	default:
		e.Section = "Same-TF"
		e.Timeframe, e.Label = sameBarLabel(id)
	}
	return e
}

func tfFieldMeta(name string) (family, unit, label string) {
	switch name {
	case "Vwema":
		return "VWEMA", "price", "VWEMA"
	case "ChanMid":
		return "Orange", "price", "Orange mid"
	case "ChanUp":
		return "Orange", "price", "Orange upper"
	case "ChanDn":
		return "Orange", "price", "Orange lower"
	case "Slope":
		return "VWEMA", "price / bar", "VWEMA slope"
	case "Width":
		return "channel", "price", "Volume-channel width"
	case "WidthChange":
		return "channel", "price / bar", "Width change"
	case "Distance":
		return "channel", "price", "VWEMA − orange mid"
	case "MidSlope":
		return "Orange", "price / bar", "Orange-mid slope"
	case "MidAccel":
		return "Orange", "price / bar²", "Orange-mid acceleration"
	case "Ema5":
		return "EMA", "price", "EMA5"
	case "Ema5Slope":
		return "EMA", "price / bar", "EMA5 slope"
	case "Ema5Accel":
		return "EMA", "price / bar²", "EMA5 acceleration"
	case "Ema12":
		return "EMA", "price", "EMA12"
	case "Ema12Slope":
		return "EMA", "price / bar", "EMA12 slope"
	case "VwemaAccel":
		return "VWEMA", "price / bar²", "VWEMA acceleration"
	case "RsiClose":
		return "RSI", "RSI", "RSI close"
	case "RsiCloseSlope":
		return "RSI", "RSI / bar", "RSI close slope"
	case "RsiCloseAccel":
		return "RSI", "RSI / bar²", "RSI close acceleration"
	case "CloseMid":
		return "close channel", "price", "Close-channel mid"
	case "CloseUp":
		return "close channel", "price", "Close-channel upper"
	case "CloseDn":
		return "close channel", "price", "Close-channel lower"
	case "CloseWidth":
		return "close channel", "price", "Close-channel width"
	case "CloseWidthChange":
		return "close channel", "price / bar", "Close-width change"
	case "Ema7":
		return "EMA", "price", "EMA7"
	case "Ema7Slope":
		return "EMA", "price / bar", "EMA7 slope"
	case "Ema7Accel":
		return "EMA", "price / bar²", "EMA7 acceleration"
	case "Macd":
		return "MACD", "price", "MACD"
	case "MacdSlope":
		return "MACD", "price / bar", "MACD slope"
	case "MacdAccel":
		return "MACD", "price / bar²", "MACD acceleration"
	default:
		return name, "", name
	}
}

func rsxFieldMeta(name string) (family, unit, label string) {
	switch name {
	case "Value":
		return "RSX", "RSX", "RSX"
	case "Slope":
		return "RSX", "RSX / bar", "RSX slope"
	case "Accel":
		return "RSX", "RSX / bar²", "RSX acceleration"
	case "Signal":
		return "RSX", "RSX", "RSX signal"
	case "SignalSlope":
		return "RSX", "RSX / bar", "RSX signal slope"
	default:
		return "RSX", "RSX", name
	}
}

func rootRSXMeta(id string) (family, unit, label string) {
	switch id {
	case "RSX":
		return "RSX", "RSX", "15m RSX"
	case "RSXSlope":
		return "RSX", "RSX / bar", "15m RSX slope"
	case "RSXAccel":
		return "RSX", "RSX / bar²", "15m RSX acceleration"
	case "Signal":
		return "RSX", "RSX", "15m RSX signal"
	case "SignalSlope":
		return "RSX", "RSX / bar", "15m RSX signal slope"
	case "RSXMinusSignal":
		return "RSX", "RSX", "15m RSX − signal"
	default:
		return "RSX", "RSX", id
	}
}

func matrixRSXTF(id string) string {
	switch {
	case strings.HasPrefix(id, "H1"):
		return "1h"
	case strings.HasPrefix(id, "H4"):
		return "4h"
	case strings.HasPrefix(id, "D1"):
		return "Daily"
	default:
		return ""
	}
}

func neighborLabel(id string) (tf, label string) {
	pair, rest := "", id
	switch {
	case strings.HasPrefix(id, "M15H1"):
		pair, rest, tf = "15m − 1h", strings.TrimPrefix(id, "M15H1"), "15m−1h"
	case strings.HasPrefix(id, "H1H4"):
		pair, rest, tf = "1h − 4h", strings.TrimPrefix(id, "H1H4"), "1h−4h"
	case strings.HasPrefix(id, "H4D1"):
		pair, rest, tf = "4h − Daily", strings.TrimPrefix(id, "H4D1"), "4h−Daily"
	}
	name := rest
	switch rest {
	case "OrangeMid":
		name = "Orange mid"
	case "Vwema":
		name = "VWEMA"
	case "CloseWidth":
		name = "Close-channel width"
	case "Width":
		name = "Volume-channel width"
	case "RsiClose":
		name = "RSI close"
	case "Rsx":
		name = "RSX"
	}
	return tf, pair + " " + name
}

func sameBarLabel(id string) (tf, label string) {
	switch {
	case strings.HasPrefix(id, "M15"):
		tf = "15m"
	case strings.HasPrefix(id, "H1"):
		tf = "1h"
	case strings.HasPrefix(id, "H4"):
		tf = "4h"
	case strings.HasPrefix(id, "D1"):
		tf = "Daily"
	}
	switch {
	case strings.Contains(id, "Ema7MinusMacd"):
		return tf, tf + " EMA7 − MACD"
	case strings.Contains(id, "Ema7MinusCloseMid"):
		return tf, tf + " EMA7 − close mid"
	case strings.Contains(id, "RsiCloseMinusCloseMid"):
		return tf, tf + " RSI close − close mid"
	case strings.Contains(id, "VwemaSlopeMinusMidSlope"):
		return tf, tf + " VWEMA slope − orange-mid slope"
	default:
		return tf, id
	}
}
