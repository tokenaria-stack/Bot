package server

import "math"

const wireFloatDecimals = 6

func roundWireFloat(v float64) float64 {
	if v == 0 {
		return 0
	}
	pow := math.Pow(10, wireFloatDecimals)
	return math.Round(v*pow) / pow
}

func roundWirePrice(v float64) float64 {
	return roundWireFloat(v)
}

func roundWireVolume(v float64) float64 {
	if v == 0 {
		return 0
	}
	return math.Round(v)
}

func compactChartOscillator(o ChartOscillator) ChartOscillator {
	o.Jurik = roundWireFloat(o.Jurik)
	o.RSX = roundWireFloat(o.RSX)
	o.RSXSignal = roundWireFloat(o.RSXSignal)
	o.Red = roundWireFloat(o.Red)
	o.Green = roundWireFloat(o.Green)
	o.RedLine = roundWireFloat(o.RedLine)
	o.GreenLine = roundWireFloat(o.GreenLine)
	o.Blue = roundWireFloat(o.Blue)
	o.RsiPrice = roundWireFloat(o.RsiPrice)
	o.EmaRsi = roundWireFloat(o.EmaRsi)
	o.RsiRsi = roundWireFloat(o.RsiRsi)
	o.RsiHl2 = roundWireFloat(o.RsiHl2)
	o.RsiVolFast = roundWireFloat(o.RsiVolFast)
	o.RsiVolSlow = roundWireFloat(o.RsiVolSlow)
	o.MacdRsi = roundWireFloat(o.MacdRsi)
	o.RsiAd = roundWireFloat(o.RsiAd)
	o.RsiHl2Vol = roundWireFloat(o.RsiHl2Vol)
	o.VolChanMid = roundWireFloat(o.VolChanMid)
	o.VolChanUp = roundWireFloat(o.VolChanUp)
	o.VolChanDn = roundWireFloat(o.VolChanDn)
	o.PriceChanMid = roundWireFloat(o.PriceChanMid)
	o.PriceChanUp = roundWireFloat(o.PriceChanUp)
	o.PriceChanDn = roundWireFloat(o.PriceChanDn)
	if o.Marker == "" {
		o.Marker = ""
	}
	if !o.VolumeSpikeUp {
		o.VolumeSpikeUp = false
	}
	if !o.VolumeSpikeDown {
		o.VolumeSpikeDown = false
	}
	return o
}
