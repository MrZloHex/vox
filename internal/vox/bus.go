package vox

import (
	"encoding/json"
	"log/slog"
	"net/url"

	"github.com/gorilla/websocket"
)

type Bus struct {
	conn *websocket.Conn
}

type BusMessage struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
	Audio   []byte `json:"audio,omitempty"`
}

func NewBus(wsURL string) (*Bus, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	slog.Info("Connected to bus", "url", wsURL)
	return &Bus{conn: conn}, nil
}

func (b *Bus) Read() (*BusMessage, error) {
	_, msg, err := b.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var m BusMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

func (b *Bus) Write(m *BusMessage) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return b.conn.WriteMessage(websocket.TextMessage, data)
}
