package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

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

// var endpoints = make(map[string]gin.HandlerFunc)
// var lock = sync.RWMutex{}
var localConn, remoteConn *websocket.Conn

func main() {
	go clientRun()

	r := gin.Default()

	// API Handler to add new client WebSocket endpoint
	r.GET("/OCPP/moxa-cp-1", serverRun)

	// Start the server in a goroutine
	go func() {
		log.Fatal(r.Run(":19000"))
	}()

	// 设置中断信号处理
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// 等待中断信号
	<-interrupt
	fmt.Println("Interrupt signal received. Closing connections...")
	if remoteConn != nil {
		remoteConn.Close()
	}
	if localConn != nil {
		localConn.Close()
	}
	time.Sleep(time.Duration(1 * time.Second))
	fmt.Println("Program exiting gracefully.")
	os.Exit(0)
}

func serverRun(c *gin.Context) {
	// check client request sub protocols
	responseHeader := http.Header{}

	clientSubprotocols := websocket.Subprotocols(c.Request)
	negotiatedSuprotocol := ""
out:
	for _, requestedProto := range clientSubprotocols {
		if len(upgrader.Subprotocols) == 0 {
			// All subProtocols are accepted, pick first
			negotiatedSuprotocol = requestedProto
			break
		}
		// Check if requested suprotocol is supported by server
		for _, supportedProto := range upgrader.Subprotocols {
			if requestedProto == supportedProto {
				negotiatedSuprotocol = requestedProto
				break out
			}
		}
	}
	if negotiatedSuprotocol != "" {
		responseHeader.Add("Sec-WebSocket-Protocol", negotiatedSuprotocol)
	}
	var err error
	localConn, err = upgrader.Upgrade(c.Writer, c.Request, responseHeader)
	if err != nil {
		fmt.Println("Upgrade error:", err)
		return
	}
	defer localConn.Close()

	localConn.SetPongHandler(func(string) error {
		log.Println("localConn pong received")
		return nil
	})
	err = localConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
	if err != nil {
		log.Println("localConn ping:", err)
		return
	}
	log.Println("localConn ping sent")
	for {
		messageType, message, err := localConn.ReadMessage()
		if err != nil {
			fmt.Println("localConn read error:", err)
			return
		}
		fmt.Printf("receive local CP messageType: %d, message: %s\n", messageType, string(message))
		if remoteConn == nil {
			fmt.Printf("continue")

			continue
		}
		err = remoteConn.WriteMessage(messageType, message)
		if err != nil {
			fmt.Println("Write error:", err)
			return
		}

	}

}

func clientRun() {
	// 定義服務器的 WebSocket 端點
	var err error
	u := url.URL{Scheme: "ws", Host: "10.123.12.111:18081", Path: "OCPP/moxa-cp-1"}
	log.Printf("connecting to %s", u.String())

	// Prepare the request header with subprotocols
	header := http.Header{}
	header.Add("Sec-WebSocket-Protocol", "ocpp1.6, ocpp1.5")

	remoteConn, _, err = websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer remoteConn.Close()

	// 設置從服務器收到 pong 消息時的處理函數
	remoteConn.SetPongHandler(func(string) error {
		log.Println("Pong received")
		return nil
	})

	done := make(chan struct{})

	// 啟動一個 goroutine 來處理接收的消息
	go func() {
		defer close(done)
		for {
			mType, message, err := remoteConn.ReadMessage()
			if err != nil {
				log.Println("remoteConn read error:", err)
				return
			}
			log.Printf("recv remoteCPO messageType: %d, message: %s", mType, message)
			if localConn == nil {
				continue
			}
			err = localConn.WriteMessage(mType, message)
			if err != nil {
				fmt.Println("Write error:", err)
				break
			}
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
			err := remoteConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				log.Println("ping:", err)
				return
			}
			log.Println("Ping sent")
		case <-interrupt:
			log.Println("Interrupt signal received, shutting down...")
			// 发送关闭消息并关闭 WebSocket 连接
			err := remoteConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second): // 等待服务器回应或超时
			}
			remoteConn.Close()
			return
		}
	}
}
