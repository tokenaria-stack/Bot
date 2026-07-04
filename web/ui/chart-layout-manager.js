/**
 * ChartLayoutManager — virtual pane geometry via LWC scaleMargins (no flex reflow).
 */
class ChartLayoutManager {
  constructor(context, chartHostEl) {
    this.context = context;
    this.host = chartHostEl;
    this.storageKey = `ilmaru_chart_layout_${context}`;

    this.state = this.load() || {
      price: { visible: true, weight: 55, scaleId: 'right' },
      wozduh: { visible: true, weight: 22, scaleId: 'wozduh' },
      rsx: { visible: true, weight: 23, scaleId: 'rsx' },
    };
    this.gutter = 0.015;
    this.minWeight = 8;
    this._splittersBound = false;
  }

  load() {
    try {
      const saved = localStorage.getItem(this.storageKey);
      if (!saved) return null;
      const parsed = JSON.parse(saved);
      if (!parsed?.price || !parsed?.wozduh || !parsed?.rsx) return null;
      return parsed;
    } catch {
      return null;
    }
  }

  save() {
    try {
      localStorage.setItem(this.storageKey, JSON.stringify(this.state));
    } catch {
      /* noop */
    }
  }

  setPaneVisibility(paneKey, isVisible, chart) {
    if (!this.state[paneKey] || this.state[paneKey].visible === isVisible) return;

    if (!isVisible) {
      this.state.price.weight += this.state[paneKey].weight;
    } else {
      this.state.price.weight = Math.max(this.minWeight, this.state.price.weight - this.state[paneKey].weight);
    }
    this.state[paneKey].visible = isVisible;

    this.save();
    if (chart) this.apply(chart);
  }

  calculateMargins() {
    const margins = {};
    let currentTop = 0.01;

    if (this.state.price.visible) {
      const pWeight = this.state.price.weight / 100;
      margins.price = { top: currentTop, bottom: 1.0 - currentTop - pWeight };
      currentTop += pWeight + this.gutter;
    }

    if (this.state.wozduh.visible) {
      const wWeight = this.state.wozduh.weight / 100;
      margins.wozduh = { top: currentTop, bottom: Math.max(0, 1.0 - currentTop - wWeight) };
      currentTop += wWeight + this.gutter;
    } else {
      margins.wozduh = { top: currentTop, bottom: currentTop };
    }

    if (this.state.rsx.visible) {
      margins.rsx = { top: currentTop, bottom: 0.01 };
    } else {
      margins.rsx = { top: currentTop, bottom: currentTop };
    }

    return margins;
  }

  apply(chart) {
    if (!chart) return;
    const margins = this.calculateMargins();

    if (margins.price) {
      chart.priceScale('right').applyOptions({ scaleMargins: margins.price, visible: true });
    }
    chart.priceScale('wozduh').applyOptions({
      scaleMargins: margins.wozduh,
      visible: this.state.wozduh.visible,
    });
    chart.priceScale('rsx').applyOptions({
      scaleMargins: margins.rsx,
      visible: this.state.rsx.visible,
    });

    this.positionSplitters();
  }

  initSplitters() {
    if (!this.host || this._splittersBound) return;
    this._splittersBound = true;

    const splitters = this.host.querySelectorAll('.pane-splitter');
    splitters.forEach((splitter) => {
      splitter.addEventListener('pointerdown', (e) => {
        e.preventDefault();
        splitter.setPointerCapture(e.pointerId);

        const boundary = splitter.dataset.boundary;
        const hostRect = this.host.getBoundingClientRect();

        const onPointerMove = (moveEvent) => {
          const clientY = moveEvent.clientY - hostRect.top;
          const pctY = (clientY / hostRect.height) * 100;
          this.resizeBoundaries(boundary, pctY);
          requestAnimationFrame(() => {
            const chartData = typeof ChartAdapter !== 'undefined'
              ? ChartAdapter.getChartHandle(this.context)
              : null;
            if (chartData?.chart) this.apply(chartData.chart);
          });
        };

        const onPointerUp = (upEvent) => {
          splitter.releasePointerCapture(upEvent.pointerId);
          splitter.removeEventListener('pointermove', onPointerMove);
          splitter.removeEventListener('pointerup', onPointerUp);
          splitter.removeEventListener('pointercancel', onPointerUp);
          this.save();
        };

        splitter.addEventListener('pointermove', onPointerMove);
        splitter.addEventListener('pointerup', onPointerUp);
        splitter.addEventListener('pointercancel', onPointerUp);
      });
    });
  }

  resizeBoundaries(boundary, pctY) {
    if (boundary === 'price-wozduh') {
      const available = this.state.price.weight + this.state.wozduh.weight;
      let newPriceW = Math.max(this.minWeight, pctY);
      let newOscW = available - newPriceW;

      if (newOscW < this.minWeight) {
        newOscW = this.minWeight;
        newPriceW = available - newOscW;
      }
      this.state.price.weight = newPriceW;
      this.state.wozduh.weight = newOscW;
    } else if (boundary === 'wozduh-rsx') {
      const totalTop = this.state.price.weight;
      const available = this.state.wozduh.weight + this.state.rsx.weight;
      let newOscW = Math.max(this.minWeight, pctY - totalTop);
      let newRsxW = available - newOscW;

      if (newRsxW < this.minWeight) {
        newRsxW = this.minWeight;
        newOscW = available - newRsxW;
      }
      this.state.wozduh.weight = newOscW;
      this.state.rsx.weight = newRsxW;
    }
  }

  positionSplitters() {
    if (!this.host) return;
    const margins = this.calculateMargins();

    const s1 = this.host.querySelector('[data-boundary="price-wozduh"]');
    const s2 = this.host.querySelector('[data-boundary="wozduh-rsx"]');

    if (s1) {
      if (this.state.wozduh.visible && margins.price) {
        s1.style.top = `${margins.price.bottom * 100}%`;
        s1.style.display = 'block';
      } else {
        s1.style.display = 'none';
      }
    }

    if (s2) {
      if (this.state.rsx.visible && this.state.wozduh.visible && margins.wozduh) {
        s2.style.top = `${margins.wozduh.bottom * 100}%`;
        s2.style.display = 'block';
      } else {
        s2.style.display = 'none';
      }
    }
  }
}

if (typeof window !== 'undefined') {
  window.ChartLayoutManager = ChartLayoutManager;
}
