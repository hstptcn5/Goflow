import { ref } from 'vue';

export class WSClient {
  constructor() {
    this.ws = null;
    this.listeners = [];
    this.status = ref('disconnected');
    this.reconnectTimer = null;
    this.shouldReconnect = true;
  }

  connect() {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }
    this.shouldReconnect = true;
    this.status.value = 'connecting';

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws`;
    const token = localStorage.getItem('GOFLOW_API_KEY');

    if (token) {
      const bytes = new TextEncoder().encode(token);
      const binary = Array.from(bytes, (byte) => String.fromCharCode(byte)).join('');
      const encoded = btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '');
      this.ws = new WebSocket(wsUrl, [`goflow.${encoded}`]);
    } else {
      this.ws = new WebSocket(wsUrl);
    }

    this.ws.onopen = () => {
      this.status.value = 'connected';
    };

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.listeners.forEach((fn) => fn(data));
      } catch (err) {
        console.error('Failed to parse WebSocket message', err);
      }
    };

    this.ws.onclose = () => {
      this.status.value = 'disconnected';
      this.ws = null;
      if (this.shouldReconnect) {
        this.reconnectTimer = setTimeout(() => this.connect(), 3000);
      }
    };

    this.ws.onerror = (err) => {
      console.error('WebSocket error', err);
      this.status.value = 'error';
    };
  }

  subscribe(listener) {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((fn) => fn !== listener);
    };
  }

  disconnect() {
    this.shouldReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.status.value = 'disconnected';
  }
}

export const wsClient = new WSClient();
