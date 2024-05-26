package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/ponyo877/hit-and-blow-cli/ayame"
	"golang.org/x/net/websocket"
)

type mmReqMsg struct {
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type mmResMsg struct {
	Type      string    `json:"type"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	mmOrigin := "127.0.0.1:8000"
	signalingOrigin := "127.0.0.1:3000"
	mmURL := url.URL{Scheme: "ws", Host: mmOrigin, Path: "/"}
	mmURLWebURL := url.URL{Scheme: "http", Host: mmOrigin, Path: "/"}
	signalingURL := url.URL{Scheme: "ws", Host: signalingOrigin, Path: "/signaling"}
	ws, err := websocket.Dial(mmURL.String(), mmURL.Scheme, mmURLWebURL.String())
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()
	now := time.Now()
	userID, _ := shortHash(now)
	reqMsg, err := json.Marshal(mmReqMsg{
		UserID:    userID,
		CreatedAt: now,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := websocket.Message.Send(ws, string(reqMsg)); err != nil {
		log.Fatal(err)
	}
	var resMsg mmResMsg
	log.Printf("Waiting match (your ID: %s)...\n", userID)
	for {
		if err := websocket.JSON.Receive(ws, &resMsg); err != nil {
			log.Fatal(err)
			break
		}
		if resMsg.Type == "MATCH" {
			log.Printf("You are matching %s, Start P2P connection\n", resMsg.UserID)
			p2pConnection(signalingURL, resMsg.RoomID, userID)
		}
	}
}

func p2pConnection(signalingURL url.URL, roomID, userID string) {
	conn := ayame.NewConnection(signalingURL.String(), roomID, ayame.DefaultOptions(), false, false)
	defer conn.Disconnect()
	var dc *webrtc.DataChannel

	conn.OnOpen(func(metadata *interface{}) {
		log.Println("Open")
		dc, err := conn.CreateDataChannel("match-making-example", nil)
		if err != nil && err != fmt.Errorf("client does not exist") {
			log.Printf("CreateDataChannel error: %v", err)
			return
		}
		log.Printf("CreateDataChannel: label=%s", dc.Label())
		scheduleSend(dc, userID)
		dc.OnMessage(onMessage(dc))
	})

	conn.OnConnect(func() {
		log.Println("Connected")
	})

	conn.OnDataChannel(func(c *webrtc.DataChannel) {
		log.Printf("OnDataChannel: label=%s", c.Label())
		if dc == nil {
			dc = c
		}
		scheduleSend(dc, userID)
		dc.OnMessage(onMessage(dc))
	})
	if err := conn.Connect(); err != nil {
		log.Fatal("Failed to connect Ayame", err)
	}
	select {}
	// waitInterrupt()
	// conn.Disconnect()
}

func shortHash(now time.Time) (string, error) {
	h := sha256.New()
	if _, err := h.Write([]byte(now.String())); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:7], nil
}

func scheduleSend(dc *webrtc.DataChannel, userID string) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			if dc != nil {
				msg := fmt.Sprintf("Message to %s", userID)
				dc.SendText(msg)
				log.Printf("👍 Label[%s]: Send: %s\n", dc.Label(), msg)
			}
		}
	}()
}

func onMessage(dc *webrtc.DataChannel) func(webrtc.DataChannelMessage) {
	return func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			log.Printf("👎 Label[%s]: Recv: %s\n", dc.Label(), (msg.Data))
		}
	}
}
