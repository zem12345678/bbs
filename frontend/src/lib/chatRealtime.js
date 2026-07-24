const OPEN = 1;

export class ChatRealtimeClient {
  constructor({ issueTicket, websocketUrl, WebSocketImpl = globalThis.WebSocket, onEvent, onState, delays = [500, 1000, 2000, 5000, 10000] }) {
    this.issueTicket = issueTicket;
    this.websocketUrl = websocketUrl;
    this.WebSocketImpl = WebSocketImpl;
    this.onEvent = onEvent || (() => {});
    this.onState = onState || (() => {});
    this.delays = delays;
    this.rooms = [];
    this.socket = null;
    this.ready = false;
    this.timer = null;
    this.attempt = 0;
    this.stopped = true;
    this.generation = 0;
  }

  start(roomNumbers = []) {
    this.stopped = false;
    this.setRooms(roomNumbers);
    this.connect();
  }

  stop({ notify = true } = {}) {
    this.stopped = true;
    this.generation += 1;
    if (this.timer !== null) clearTimeout(this.timer);
    this.timer = null;
    const socket = this.socket;
    this.socket = null;
    this.ready = false;
    if (socket) socket.close();
    if (notify) this.onState("disconnected");
  }

  setRooms(roomNumbers = []) {
    this.rooms = [...new Set(roomNumbers.map((room) => String(room || "").trim().toUpperCase()).filter(Boolean))];
    if (this.ready) this.send("room.subscribe", { room_numbers: this.rooms });
  }

  send(type, payload = {}, requestId = "") {
    if (!this.isOpen()) return false;
    this.socket.send(JSON.stringify({ type, request_id: requestId || undefined, payload }));
    return true;
  }

  isOpen() {
    return Boolean(this.ready && this.socket && this.socket.readyState === OPEN);
  }

  async connect() {
    if (this.stopped || !this.WebSocketImpl) return;
    const generation = ++this.generation;
    this.onState(this.attempt ? "reconnecting" : "connecting");
    let ticketData;
    try {
      ticketData = await this.issueTicket();
    } catch (error) {
      if (generation === this.generation) this.scheduleReconnect(error);
      return;
    }
    if (this.stopped || generation !== this.generation) return;
    let socket;
    try {
      socket = new this.WebSocketImpl(this.websocketUrl(ticketData?.ticket || ticketData));
    } catch (error) {
      this.scheduleReconnect(error);
      return;
    }
    this.socket = socket;
    this.ready = false;
    socket.onopen = () => {
      if (generation !== this.generation || this.stopped) return;
      this.onState("authenticating");
    };
    socket.onmessage = (message) => {
      if (generation !== this.generation || this.stopped) return;
      try {
        const event = JSON.parse(message.data);
        this.onEvent(event);
        if (event.type === "session.ready") {
          this.ready = true;
          this.attempt = 0;
          this.onState("connected");
          this.send("room.subscribe", { room_numbers: this.rooms });
        }
      } catch {
        this.onState("protocol_error");
      }
    };
    socket.onerror = () => {
      if (generation === this.generation) this.onState("reconnecting");
    };
    socket.onclose = () => {
      if (generation !== this.generation || this.stopped) return;
      this.socket = null;
      this.ready = false;
      this.scheduleReconnect();
    };
  }

  scheduleReconnect(error) {
    if (this.stopped || this.timer !== null) return;
    this.onState("reconnecting");
    const delay = this.delays[Math.min(this.attempt, this.delays.length - 1)] || 10000;
    this.attempt += 1;
    this.timer = setTimeout(() => {
      this.timer = null;
      this.connect();
    }, delay);
    if (error) this.lastError = error;
  }
}
