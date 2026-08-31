/**
 * WebSocket transport — routes ticks/markers/timeline signals to callbacks, no Store/DOM.
 */
const WS = {
  _socket: null,
  _callbacks: null,
  _reconnectTimer: null,
  _subscribedTf: null,
  _slots: null,
  _facts: null,

  isOpen() {
    return WS._socket?.readyState === WebSocket.OPEN;
  },

  connect(callbacks = {}) {
    WS._callbacks = callbacks;
    WS._openSocket();
  },

  disconnect() {
    if (WS._reconnectTimer) {
      clearTimeout(WS._reconnectTimer);
      WS._reconnectTimer = null;
    }
    if (WS._socket) {
      WS._socket.onopen = null;
      WS._socket.onclose = null;
      WS._socket.onerror = null;
      WS._socket.onmessage = null;
      if (WS._socket.readyState === WebSocket.OPEN || WS._socket.readyState === WebSocket.CONNECTING) {
        WS._socket.close();
      }
      WS._socket = null;
    }
  },

  subscribe(tf, resolvedTf, slots, facts) {
    // Case-sensitive: "1m" (minute) ≠ "1M" (month).
    const subTf = String(resolvedTf ?? tf ?? '');
    if (!subTf) return;
    WS._subscribedTf = subTf;
    if (Array.isArray(slots)) WS._slots = slots.slice();
    if (Array.isArray(facts)) WS._facts = facts.slice();
    WS._sendSubscribe(subTf);
  },

  _sendSubscribe(tf) {
    const send = () => {
      if (WS._socket && WS._socket.readyState === WebSocket.OPEN) {
        const payload = {
          type: 'subscribe',
          tf,
          ...(Array.isArray(WS._slots) ? { slots: WS._slots } : {}),
        };
        // Array (including []) is explicit demand. Omit only when never set (legacy ALL).
        if (Array.isArray(WS._facts)) payload.facts = WS._facts;
        WS._socket.send(JSON.stringify(payload));
      }
    };
    send();
    if (WS._socket && WS._socket.readyState === WebSocket.CONNECTING) {
      WS._socket.addEventListener('open', send, { once: true });
    }
  },

  _openSocket() {
    WS.disconnect();
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    WS._socket = new WebSocket(`${proto}://${location.host}/ws`);

    WS._socket.onopen = () => {
      if (WS._subscribedTf) {
        WS._sendSubscribe(WS._subscribedTf);
      }
      WS._callbacks?.onOpen?.();
    };

    WS._socket.onmessage = (event) => {
      WS._dispatchMessage(event);
    };

    WS._socket.onerror = () => {
      WS._callbacks?.onError?.();
    };

    WS._socket.onclose = () => {
      WS._callbacks?.onClose?.();
      WS._reconnectTimer = setTimeout(() => {
        WS._reconnectTimer = null;
        WS._callbacks?.onReconnect?.();
        WS._openSocket();
      }, 3000);
    };
  },

  _dispatchMessage(event) {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch {
      return;
    }

    if (msg.type === 'tick' && msg.data) {
      // Case-sensitive TF gate (defense-in-depth; server routeTick is SSOT).
      const tickTf = String(msg.data.timeframe || '');
      if (WS._subscribedTf && tickTf && tickTf !== WS._subscribedTf) return;
      WS._callbacks?.onTick?.(msg.data);
      return;
    }

    if (msg.type === 'marker' && msg.data) {
      WS._callbacks?.onMarker?.(msg.data);
      return;
    }

    // Timeline publish gate (Phase C/D): server owns heal; FE waits then reloads.
    if (msg.type === 'timeline_healing') {
      WS._callbacks?.onTimelineHealing?.(msg.data ?? msg);
      return;
    }
    if (msg.type === 'timeline_publishable') {
      WS._callbacks?.onTimelinePublishable?.(msg.data ?? msg);
    }
  },
};

if (typeof window !== 'undefined') {
  window.WS = WS;
}
