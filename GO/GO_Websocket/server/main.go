package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允許跨域
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("Upgrade error:", err)
			return
		}
		defer conn.Close()

		conn.SetPongHandler(func(string) error {
			log.Println("Pong received")
			return nil
		})

		for {
			// 讀取客戶端發送的消息
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("Read error:", err)
				break
			}
			fmt.Printf("receive local CP meesageType: %d, message: %s", messageType, message)

			// 將消息回發給客戶端
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Println("Write error:", err)
				break
			}

		}
	})

	fmt.Println("WebSocket server starting on :18081")
	http.ListenAndServe(":18081", nil)
}
