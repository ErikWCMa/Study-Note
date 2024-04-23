package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// 定義服務器的 WebSocket 端點

	u := url.URL{Scheme: "ws", Host: "127.0.0.1:19000", Path: "OCPP/moxa-cp-1"}
	log.Printf("connecting to %s", u.String())

	// Prepare the request header with subprotocols
	header := http.Header{}
	header.Add("Sec-WebSocket-Protocol", "ocpp1.6, ocpp1.5")

	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	// 設置從服務器收到 pong 消息時的處理函數
	c.SetPongHandler(func(string) error {
		log.Println("Pong received")
		return nil
	})

	done := make(chan struct{})

	// 啟動一個 goroutine 來處理接收的消息
	go func() {
		defer close(done)
		for {
			mType, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				return
			}
			log.Printf("recv messageType: %d, message: %s", mType, message)
		}
	}()
	// 设置接收中断信号
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// 定期发送 ping 消息
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			err := c.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				log.Println("ping:", err)
				return
			}
			log.Println("Ping sent")
		case <-interrupt:
			log.Println("Interrupt signal received, shutting down...")
			// 发送关闭消息并关闭 WebSocket 连接
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second): // 等待服务器回应或超时
			}
			c.Close()
			return
		}
	}
}
