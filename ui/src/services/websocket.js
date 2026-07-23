export class WSClient {
  constructor() {
    this.ws = null;
    this.listeners = [];
  }

  connect() {
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
      console.log('⚡ Connected to Goflow Real-time WebSocket');
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
      console.log('🔌 WebSocket disconnected, reconnecting in 3s...');
      setTimeout(() => this.connect(), 3000);
    };

    this.ws.onerror = (err) => {
      console.error('WebSocket error', err);
    };
  }

  subscribe(listener) {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((fn) => fn !== listener);
    };
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
    }
  }
}

export const wsClient = new WSClient();
