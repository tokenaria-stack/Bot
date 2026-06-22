# Pine Script Archive — TradingView → Go

Справочник оригинальных исходников индикаторов TradingView (Pine Script v3), которые переносим в бот на Go.  
Используем для сверки математики, регрессионных тестов и предложений по улучшению логики.

| # | Индикатор | Pine shorttitle | Go-реализация | Статус |
|---|-----------|-----------------|---------------|--------|
| 1 | Jurik RSX (everget) | `RSX` | `indicators/jurik.go`, `strategy/rsx_chart.go` | ✅ core ported |
| 2 | RSIVolume Wozduh | `RSIVol_2graf.02` | `strategy/falcon.go`, `indicators/rsi.go`, `indicators/ema.go` | ✅ core ported |
| 3 | Trendline Breakout Navigator | `LuxAlgo - Trendline Breakout Navigator` | `indicators/geometry.go`, `strategy/geometry_tracker.go` (partial) | 🔜 planned |

**Сырые `.pine` файлы:** каталог [`pine/`](../pine/) (полные исходники для diff и портирования).

---

## 1. Jurik RSX — Alex Orekhov (everget)

**TradingView:** `study("Jurik RSX", shorttitle="RSX")`  
**Лицензия:** MIT (Copyright 2019-present, Alex Orekhov / everget)  
**Назначение в боте:** главный осциллятор 0–100, цвет линии, TV rolling divergence (режим `div_method: "tv"`).

### Ключевые входы Pine → Go

| Pine input | Default | Go mapping |
|------------|---------|------------|
| `length` | 14 | `RSXSettings.Length` / `JurikRSX` |
| `src` | `hlc3` | `RSXSettings.Source` (`"close"` \| `"hlc3"`) |
| `obLevel` / `osLevel` | 70 / 30 | UI зоны; в боте цвет: 50 + slope (см. `rsx_chart.go`) |
| `xbars` | 90 | `RSXSettings.DivLookback` (default **90**) |
| Rolling div | `highestbars` / `lowestbars` | `strategy/rsx_div_tv.go` (Phase 2 restore) |
| Pivot labels | offset −2 | Fractal mode: `RSXSettings.PivotRadius` = **2** (5-bar pivot) |

### Полный исходник Pine Script v3

```pine
//@version=3
// Copyright (c) 2019-present, Alex Orekhov (everget)
// Jurik RSX script may be freely distributed under the MIT license.
study("Jurik RSX", shorttitle="RSX")

length = input(title="Length", type=integer, defval=14)
src = input(title="Source", type=source, defval=hlc3)
obLevel = input(title="OB Level", type=integer, defval=70)
osLevel = input(title="OS Level", type=integer, defval=30)
highlightBreakouts = input(title="Highlight Overbought/Oversold Breakouts ?", type=bool, defval=true)

f8 = 100 * src
f10 = nz(f8[1])
v8 = f8 - f10

f18 = 3 / (length + 2)
f20 = 1 - f18

f28 = 0.0
f28 := f20 * nz(f28[1]) + f18 * v8

f30 = 0.0
f30 := f18 * f28 + f20 * nz(f30[1])
vC = f28 * 1.5 - f30 * 0.5

f38 = 0.0
f38 := f20 * nz(f38[1]) + f18 * vC

f40 = 0.0
f40 := f18 * f38 + f20 * nz(f40[1])
v10 = f38 * 1.5 - f40 * 0.5

f48 = 0.0
f48 := f20 * nz(f48[1]) + f18 * v10

f50 = 0.0
f50 := f18 * f48 + f20 * nz(f50[1])
v14 = f48 * 1.5 - f50 * 0.5

f58 = 0.0
f58 := f20 * nz(f58[1]) + f18 * abs(v8)

f60 = 0.0
f60 := f18 * f58 + f20 * nz(f60[1])
v18 = f58 * 1.5 - f60 * 0.5

f68 = 0.0
f68 := f20 * nz(f68[1]) + f18 * v18

f70 = 0.0
f70 := f18 * f68 + f20 * nz(f70[1])
v1C = f68 * 1.5 - f70 * 0.5

f78 = 0.0
f78 := f20 * nz(f78[1]) + f18 * v1C

f80 = 0.0
f80 := f18 * f78 + f20 * nz(f80[1])
v20 = f78 * 1.5 - f80 * 0.5

f88_ = 0.0
f90_ = 0.0

f88 = 0.0
f90_ := nz(f90_[1]) == 0 ? 1 : nz(f88[1]) <= nz(f90_[1]) ? nz(f88[1]) + 1 : nz(f90_[1]) + 1
f88 := nz(f90_[1]) == 0 and (length - 1 >= 5) ? length - 1 : 5

f0 = f88 >= f90_ and f8 != f10 ? 1 : 0
f90 = f88 == f90_ and f0 == 0 ? 0 : f90_

v4_ = f88 < f90 and v20 > 0 ? (v14 / v20 + 1) * 50 : 50
rsx = v4_ > 100 ? 100 : v4_ < 0 ? 0 : v4_

rsxColor = rsx > obLevel ? #0ebb23 : rsx < osLevel ? #ff0000 : #512DA8
plot(rsx, title="RSX", linewidth=2, color=rsxColor, transp=0)

transparent = color(white, 100)

maxLevelPlot = hline(100, title="Max Level", linestyle=dotted, color=transparent)
obLevelPlot = hline(obLevel, title="Overbought Level", linestyle=dotted)
hline(50, title="Middle Level", linestyle=dotted)
osLevelPlot = hline(osLevel, title="Oversold Level", linestyle=dotted)
minLevelPlot = hline(0, title="Min Level", linestyle=dotted, color=transparent)

fill(obLevelPlot, osLevelPlot, color=purple, transp=95)

obFillColor = rsx > obLevel and highlightBreakouts ? green : transparent
osFillColor = rsx < osLevel and highlightBreakouts ? red : transparent

fill(maxLevelPlot, obLevelPlot, color=obFillColor, transp=90)
fill(minLevelPlot, osLevelPlot, color=osFillColor, transp=90)

piv = input(false,"Hide pivots?")
shrt = input(false,"Shorter labels?")
xbars = input(90, "Div lookback period (bars)?", integer, minval=1)
hb = abs(highestbars(rsx, xbars)) // Finds bar with highest value in last X bars
lb = abs(lowestbars(rsx, xbars)) // Finds bar with lowest value in last X bars
max = na
max_rsi = na
min = na
min_rsi = na
pivoth = na
pivotl = na
divbear = na
divbull = na

// If bar with lowest / highest is current bar, use it's value
max := hb == 0 ? close : na(max[1]) ? close : max[1]
max_rsi := hb == 0 ? rsx : na(max_rsi[1]) ? rsx : max_rsi[1]
min := lb == 0 ? close : na(min[1]) ? close : min[1]
min_rsi := lb == 0 ? rsx : na(min_rsi[1]) ? rsx : min_rsi[1]

// Compare high of current bar being examined with previous bar's high
// If curr bar high is higher than the max bar high in the lookback window range
if close > max // we have a new high
    max := close // change variable "max" to use current bar's high value
if rsx > max_rsi // we have a new high
    max_rsi := rsx // change variable "max_rsi" to use current bar's RSI value
if close < min // we have a new low
    min := close // change variable "min" to use current bar's low value
if rsx < min_rsi // we have a new low
    min_rsi := rsx // change variable "min_rsi" to use current bar's RSI value

// Finds pivot point with at least 2 right candles with lower value
pivoth := (max_rsi == max_rsi[2]) and (max_rsi[2] != max_rsi[3]) ? true : na
pivotl := (min_rsi == min_rsi[2]) and (min_rsi[2] != min_rsi[3]) ? true : na

// Detects divergences between price and indicator with 1 candle delay so it filters out repeating divergences
if (max[1] > max[2]) and (rsx[1] < max_rsi) and (rsx <= rsx[1])
    divbear := true
if (min[1] < min[2]) and (rsx[1] > min_rsi) and (rsx >= rsx[1])
    divbull := true

// Alerts
alertcondition(divbear, title='Bear div', message='Bear div')
alertcondition(divbull, title='Bull div', message='Bull div')

// Plots divergences and pivots with offest
// Longer labels
plotshape(shrt ? na : divbear ? rsx[1] + 1 : na, location=location.absolute, style=shape.labeldown, color=red, size=size.tiny, text="Bear", textcolor=white, transp=0, offset=-1)
plotshape(shrt ? na : divbull ? rsx[1] - 1 : na, location=location.absolute, style=shape.labelup, color=green, size=size.tiny, text="Bull", textcolor=white, transp=0, offset=-1)
plotshape(piv ? na : shrt ? na : pivoth ? max_rsi + 1 : na, location=location.absolute, style=shape.labeldown, color=blue, size=size.tiny, text="Pivot", textcolor=white, transp=0, offset=-2)
plotshape(piv ? na : shrt ? na : pivotl ? min_rsi - 1 : na, location=location.absolute, style=shape.labelup, color=blue, size=size.tiny, text="Pivot", textcolor=white, transp=0, offset=-2)

// Shorter labels
plotshape(shrt ? (divbear ? rsx[1] + 3 : na) : na, location=location.absolute, style=shape.triangledown, color=red, size=size.tiny, transp=0, offset=-1)
plotshape(shrt ? (divbull ? rsx[1] - 3 : na) : na, location=location.absolute, style=shape.triangleup, color=green, size=size.tiny, transp=0, offset=-1)
plotshape(piv ? na : shrt ? (pivoth ? max_rsi + 3 : na) : na, location=location.absolute, style=shape.triangledown, color=blue, size=size.tiny, transp=0, offset=-2)
plotshape(piv ? na : shrt ? (pivotl ? min_rsi - 3 : na) : na, location=location.absolute, style=shape.triangleup, color=blue, size=size.tiny, transp=0, offset=-2)
```

### Заметки для сверки с Go

- **Ядро RSX:** `f18 = 3/(length+2)`, двойное сглаживание `f28/f30 → vC → … → v14/v20`, warmup-счётчик `f88/f90`, итог `(v14/v20+1)*50` — см. `indicators/jurik.go`.
- **Дивергенции TV:** rolling `max`/`min` по **close** (не high/low), окно `xbars`, задержка 1 бар (`offset=-1` на метках). Маркеры Pine: `Bear` / `Bull` → в боте `S` / `L`.
- **Пивоты Pine:** условие `max_rsi == max_rsi[2]` (2 бара справа ниже) — близко к нашему fractal radius=2, но логика иная (на истории max_rsi, не локальный экстремум RSX).
- **Цвет в Pine:** OB/OS 70/30. В боте: rising+fallback 50 / falling+below 50 (`rsx_chart.go`) — осознанное отличие для дашборда.

---

## 2. RSIVolume Wozduh — @wozdux

**TradingView:** `study(title="RSIVolume_2graf.02[wozdux]", shorttitle="RSIVol_2graf.02")`  
**Автор:** @wozdux  
**Назначение в боте:** панель Wozduh / Falcon — объёмный RSI, каналы, кроссы, вспомогательные линии для скоринга.

### Ключевые входы Pine → Go (`strategy/falcon.go`)

| Pine variable | Default | Go field / const |
|---------------|---------|------------------|
| `lenvol` | 24 | `wozduhLenVol`, `wozduhChannelPeriod` |
| `lencena` | 14 | RSI(close) → `RsiPrice` / `RedLine` |
| `lenn` | 14 | RSI(HL2) → `RsiHl2` / `RedLine` legacy |
| `lenOR` | 14 | RSI(RSI) → `RsiRsi` |
| `ll` | 7 | EMA(RSI) → `EmaRsi` / `GreenLine` |
| `oo1` / `oo2` | 12 / 5 | `wozduhWt11Period` / `wozduhWt22Period` → `RsiVolFast` / `RsiVolSlow` |
| `cek` | 0.6 | inner channel (не портирован в UI) |
| `ko3` | 3 | HL2 smoothed range (chl3) — частично |
| `mrsi`, `m1`, `m2`, `mmn` | 24/7/24/1 | MACD(RSI)+50 → `MacdRsi` / `BlackLine` |
| Channel φ | `1.6185 * stdev` | `wozduhChannelPhi` |
| `cross(wt11, wt22)` | circles | `VolCrossMarker` lime/red |

### Полный исходник Pine Script v3

```pine
//@version=3
// @wozdux
// гладкие линии -это rsi от обычной цены
// красная линия=это самая быстрая. Она показывает  интенсинвость закупок по сравнению с продажами
// прерывистые линии показывают rsi по объемной цене.АА
// фактически rsi-это интенсивность закупок
// поэтому объемная rsi это включение объема в импульс.
study(title="RSIVolume_2graf.02[wozdux] ", shorttitle="RSIVol_2graf.02")
lenvol = input(24, minval=1,title="BLUE=rsi1(close*volume,lenvol)")
lencena = input(14, minval=1, title="RED=rsi3(close)")
lenn = input(14, minval=1,title="ФИОЛЕТ=lenn=(H+L)/2")

lenOR = input(14, minval=1, title="ORANGE=rsi(rsi3(close))")
ll = input(7, minval=1, title="green=ema(rsi,ll)")
uplimit = input(70, minval=1,title="level_close")
urvol = input(8, minval=1,title="level_volume*close")
dada = input(true,  title="ГОЛУБОЙ=RSI(vol)=включить канал")
qdada = input(false,  title="КРАСНЫЙ=RSI(cana)= включить канал")

klrscena = input(true, title="КРАСНЫЙ=включить rsi(H+L)/2")
klrsirsi = input(false, title="ОРАНЖ=включить rsirsi")
klemarsi = input(false, title="ЗЕЛЕН=включить EMA(RSI)")
klnavy = input(false, title="ТЕМНО-СИНИЙ=RSI(HL2)")
mrsi = input(24, minval=1,title="ЧЕРНЫЙ=macd(rsi)")
m1 = input(7, minval=1,title="black=MACD(rsi)")
m2 = input(24, minval=1, title="black=MACD2")
mmn = input(1,  title=" множитель для МАКД")
klmacd = input(false, title="key MACD")
dnlimit=100-uplimit
cena = close


ko3 = input(3, minval=1,title="ko3=4* (H+L)/2")
cc=(high+low)/2
chh=highest(cc,ko3)
cll=lowest(cc,ko3)
chl3=(chh+cll)/2
rsch2=rsi(cc,lenn)
rsch3=rsi(chl3,lenn)
plot(klrscena ? rsch2 : na, color=purple, style=line,linewidth=2, title="rsi(HL2,len)")
//plot(rsch3, color=lime, style=line,linewidth=2, title="rsi(HL2,len*ko3)")
// БЕРЕМ ЗА ОСНОВУ ОБЪЕМНУЮ ЦЕНУ
/////////////////////////////////////////////////////
// ОБЪЕМНАЯ ЦЕНА ЗА ДЛИННЫЙ ПЕРИОД=СРЕДНЯЯ ЦЕНА НА ЕДИНИЦУ ОБЪЕМА
aaa0 = ema( volume * cena, lenvol ) / ema( volume, lenvol ) 
aaacc = ema( volume * cc, lenvol ) / ema( volume, lenvol ) 
aaahl = ema( volume * chl3, lenvol ) / ema( volume, lenvol ) 
srvolum=ema(volume,lenvol)
aaa1=aaa0 


/////////объемная цена/////////////////////
oo1 = input(12,  title=" СГЛАЖИВАНИЕ RSI-СИНИЙ")
oo2 = input(5,  title=" СГЛАЖИВАНИЕ rsi ГОЛУБОЙ")
//oo1=12,oo2=12
rsi11 = rsi(aaa1, lenvol)
emarr=ema(rsi11,oo1)
////////////// ОБЪЕМНАЯ ЦЕНА ЛЪЕМНАЯ ЦЕНА RSI RSI
wt11=emarr
wt22=ema(rsi11,oo2)
plot(wt22, color=aqua, style=line,linewidth=2, title="rsi(close*volume,lenvol)")
plot(emarr, color=blue, style=line,linewidth=2, title="rsi(close*volume,lenvol)")
//plot(rsi11, color=black, style=line,linewidth=2, title="rsi(close*volume,lenvol)")
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////
cek = input(0.6,  title="ГОЛУБОЙ= внутренниq КАНАЛ")

r=wt22, lband=lenvol
///////////////////////
ma=sma(r,lband)
offs=(1.6185 * stdev(r, lband))
up=ma+offs
upww=ma+offs*cek
dn=ma-offs
/// ОКРАСКА КАНАЛА
/// ОКРАСКА КАНАЛА
/// ОКРАСКА КАНАЛА
/// ОКРАСКА КАНАЛА
plot(ma, color=orange, linewidth=2,title="seredina")
midl=plot(dada ? ma: na, color=orange, linewidth=2,title="seredina")
upl00=plot(dada ? up: na , color=blue,title="kanal")
dnl00=plot(dada ? dn : na, color=blue,title="kanal")
uplww=plot(dada ? upww : na, color=blue,title="kanal")
fill(midl, dnl00, green, transp=90)
fill(midl, upl00, aqua, transp=90)
///КОНЕЦ ОПИСАНИЯ КАНАЛА

plot(cross(wt11, wt22) ? wt22 : na, color = black , style = circles, linewidth = 3,title="cross")
plot(cross(wt11, wt22) ? wt22 : na, color = (wt22 - wt11 > 0 ? red : lime) , style = circles, linewidth = 2,title="cross")
////////////////////////////////////////
rsiaacc = rsi(aaacc, lenvol)
plot(klnavy ? rsiaacc : na, color=navy, style=line,linewidth=3, title="rsi(HL2*volume,lenvol)")
rsihl = rsi(aaahl, lenvol)
//plot(rsihl, color=green, style=line,linewidth=3, title="rsi(HL2*volume,lenvol)")
//////////обычная цена////////////////////////////////
//////////обычная цена////////////////////////////////
//////////обычная цена////////////////////////////////
//////////обычная цена////////////////////////////////
rsi3 = rsi(cena, lencena)
rs3=plot(klrscena ? rsi3 : na, color=red, linewidth=2 , title="rsi(close,lencena)")
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////
/////КАНАЛ КАННАЛал////////////////////////////////

qr=rsi3, qlband=lenvol
///////////////////////
qma=sma(qr,qlband)
qoffs=(1.6185 * stdev(qr, qlband))
qup=qma+qoffs
qdn=qma-qoffs
/// ОКРАСКА КАНАЛА
qmidl=plot(qdada ? qma : na, color=maroon, linewidth=2,title="seredina")
qupl00=plot(qdada ? qup : na, color=blue,title="kanal")
qdnl00=plot(qdada ? qdn : na, color=blue,title="kanal")
fill(qmidl, qdnl00, yellow, transp=90)
fill(qmidl, qupl00, red, transp=90)
///КОНЕЦ ОПИСАНИЯ КАНАЛА
///КОНЕЦ ОПИСАНИЯ КАНАЛА
///КОНЕЦ ОПИСАНИЯ КАНАЛА
///КОНЕЦ ОПИСАНИЯ КАНАЛА



rsi4 = rsi(rsi3, lenOR)
plot(klrsirsi ? rsi4 : na, color=orange, linewidth=2 , title="rsi(rsi(close,lencena))")
///// MACD(RSI)
mmrsi=rsi(cena,mrsi)
ee1=ema(mmrsi,m1)
ee2=ema(mmrsi,m2)
macd=(ee1-ee2)*mmn+50
plot(klmacd ? macd : na, color=black, linewidth=2 , title="MACD(rsi)")
/////////EMA(RSI)
emr3=ema(rsi3,ll)
plot(klemarsi ? emr3 :na, color=green, linewidth=2 , title="ema(rsi(cena),ll)")
/////////раскрашиваем уровни/////////////////////////
/////////раскрашиваем уровни/////////////////////////
/////////раскрашиваем уровни/////////////////////////
/////////раскрашиваем уровни/////////////////////////
uu = hline(urvol, linestyle=dotted,title="color70")
uu0 = hline(urvol-3, linestyle=solid, title="color70")
fill(uu, uu0, color=yellow, transp=80)
vv = hline(100-urvol, linestyle=dotted,title="color70")
vv0 = hline(100-urvol-3, linestyle=solid, title="color70")
fill(vv, vv0, color=yellow, transp=80)
//////////////////////////////////////////////////////
```

### Заметки для сверки с Go

- **Объёмная цена:** `VWEMA(price) = EMA(volume×price)/EMA(volume)` — `indicators` VWEMA / `FalconEngine`.
- **Голубая/синяя пара:** `rsi11 = rsi(aaa1, lenvol)`, `wt11 = ema(rsi11, 12)`, `wt22 = ema(rsi11, 5)` — кросс даёт сигнал spike в скоринге (`useWozduhCross`, `useWozduhSpike`).
- **Канал объёма:** `SMA(wt22, 24) ± 1.6185·σ` — `VolChanMid/Up/Dn`.
- **Канал цены:** то же на `rsi(close, 14)` — `PriceChanMid/Up/Dn`.
- **Не портировано в UI:** `upww` (inner channel `cek=0.6`), `rsch3`, `rsihl`, level fills `urvol`.

---

## 3. Trendline Breakout Navigator — LuxAlgo

**TradingView:** `indicator('Trendline Breakout Navigator [LuxAlgo]', shorttitle='LuxAlgo - Trendline Breakout Navigator')`  
**Лицензия:** [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/) (© LuxAlgo) — **NonCommercial**; для prod-бота уточнить лицензию.  
**Исходник:** [`pine/luxalgo_trendline_breakout_navigator.pine`](../pine/luxalgo_trendline_breakout_navigator.pine)  
**Назначение (план):** динамические трендлинии HH/LL, пробои, wick-dots, MTF — дополнение/замена части `geometry_tracker`.

### Ключевые входы Pine → Go (draft)

| Pine input | Default | Смысл |
|------------|---------|--------|
| `res` | `''` (chart TF) | HTF для pivot через `request.security` |
| `l1` / `l2` / `l3` | 60 / 30 / 10 | Left bars pivot: Long / Medium / Short |
| pivot right | **1** (hardcoded) | Быстрое подтверждение swing (1 бар справа) |
| `term` | `Long` | Какой слой активен для HH/LL labels и bg |
| `i1/i2/i3` | true | Toggle каждого слоя |
| `cBull` / `cBear` | `#089981` / `#f23645` | Цвета TL (совпадают с TV palette бота) |
| `cWickBull` / `cWickBear` | `#085def` / `#ff5d00` | Wick-break dots |
| `HHLL` | `None` | HH/LL labels + optional prev H/L dot |

### Архитектура алгоритма (кратко)

1. **Три параллельных движка** — `draw(i1, res, l1, 1, 1)` ×3 с разной длиной pivot (60/30/10).
2. **MTF pivots** — `ta.pivothigh(left, 1)` / `pivotlow` на `res` через `request.security(..., [time[2], ph_])`; события `chH`/`chL` по `ta.change(fixnan(ph))`.
3. **State machine `trend`** — `1` bullish (после HH), `-1` bearish (после LL); линия строится от **противоположного** pivot (bull TL от `prevPl`, bear TL от `prevPh`).
4. **UDT `bin`** — активная линия: `{ line, slope, active, cp }`; каждый бар `set_xy2(n, y2 + slope)` (линия «течёт»).
5. **HH / LL** — новый экстремум vs `prevPh`/`prevPl`, min distance 5 bars, max lookback 5000.
6. **Wick break vs close break:**
   - Wick пробивает TL, **close** по «неправильной» стороне → **dot** (wick signal), pivot линии (LH/HL), пересчёт slope.
   - **Close** на swing ломает TL → `bn.active := false` (**breakout**).
7. **Первый swing после создания линии (`slope == 0`)** — `while` loop: сдвигает anchor (`cp`) пока внутри истории есть close, пробивающий линию (bear: max `close[i]-line`; bull: max `line-close[i]`).
8. **Outputs** — `[trend, bn.lin.get_y2()]` ×3 → скрытые plots `value1/2/3` для barcolor/bg (close vs уровень TL).

### Сравнение с текущим Go (`geometry_tracker` + `indicators/geometry.go`)

| Аспект | LuxAlgo Pine | Текущий бот |
|--------|--------------|-------------|
| Swing detection | `pivothigh/low(left, 1)` на HTF | ZigZag nodes (ATR + fractals) |
| Trendline | Динамическая, 2 anchor + rolling slope | 2 последних swing peak, статичная до нового node |
| Breakout | Close на pivot vs line → deactivate | Volume-confirmed close break (`CheckBreakout`) |
| Wick signal | Dot + pivot line (LH/HL) | Bounce detection (`BounceUp/Down`) |
| Layers | 3 (Long/Medium/Short) | 1 resistance + 1 support |
| Scoring hook | — | `IsBullishBreakout` +30, `GeometryBounce` +25 |

**Вывод:** пересечение по теме (trendlines/breakout), но **алгоритм другой**. LuxAlgo богаче сигналами (HH/LL, wick dots, triple term); наш geometry проще и уже в scoring matrix.

### Заметки для интеграции в Go

- **Streaming engine:** портировать `draw()` как `LuxAlgoTrendlineEngine` с `Update(bar)` → events: `HH`, `LL`, `WickBreak`, `TrendlineBreakout`, `TrendlineLevel`.
- **MTF:** `res` + pivot time `tH`/`tL` → использовать `ChiefAnalyst[res]` или агрегированные klines; цикл `for i = 0 to 5000` заменить бинарным поиском по `OpenTime`.
- **Индексация:** Pine `bar_index` / `n - idx` → наш `barIndex` в kline cache (как в RSX chart).
- **Dashboard:** overlay lines на price chart backtest/live (отдельная series или canvas); markers HH/LL / wick dots.
- **Scoring:** новые toggles в `scoring_matrix.go` (`useLuxAlgoTLBreakout`?) или расширить `useGeometry`.
- **Лимиты:** `max_bars_back=5000`, max line length 5000 — зеркалить в Go ring/history cap.
- **Лицензия NC:** использование в коммерческом боте может требовать отдельного разрешения LuxAlgo.

### Полный исходник

См. [`pine/luxalgo_trendline_breakout_navigator.pine`](../pine/luxalgo_trendline_breakout_navigator.pine) — Pine Script v5, без сокращений.

---

## Changelog

| Дата | Изменение |
|------|-----------|
| 2026-06-01 | LuxAlgo Trendline Breakout Navigator — исходник + анализ для интеграции |
| 2026-06-01 | Создан архив: Jurik RSX (everget) + Wozduh RSIVol_2graf.02 |
