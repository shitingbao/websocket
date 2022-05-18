package main

import (
	"context"
	"encoding/json"
	"log"
	"time"
	ws "websocket"

	"github.com/gin-gonic/gin"
)

var (
	userFlag = "protocol" // Sec-WebSocket-Protocol 的标识，可自定义，可以写在配置文件中，对应内容应该写入唯一标识符
	sendCap  = 256        // 发送的消息容量大小
)

// 结合 gin 的一个基本的 exanple，其他方法同理
func main() {
	h := ws.NewHub(func(data []byte, hub *ws.Hub) error {
		msg := ws.Message{}
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		//原样消息发公告
		hub.Broadcast <- msg
		return nil
	})
	go h.Run(context.TODO())

	go func() {
		// 发送一个全家广播消息
		time.Sleep(time.Second * 5)
		log.Println("send")
		da, _ := json.Marshal("this is data")

		h.Broadcast <- ws.Message{
			Data:     da,
			DataType: 1,
			DateTime: time.Now().Format("2006-01-02 15:04:05"),
		}
	}()

	go func() {
		// 发送一个用户消息
		time.Sleep(time.Second * 5)
		log.Println("send usertest")
		da, _ := json.Marshal("this is usertest data")
		h.Broadcast <- ws.Message{
			UserFlag: "usertest",
			Data:     da,
			DataType: 1,
			DateTime: time.Now().Format("2006-01-02 15:04:05"),
		}
	}()

	g := gin.Default()
	g.GET("/ws", func(ctx *gin.Context) {
		ws.ServeWs(context.Background(), userFlag, sendCap, h, ctx.Writer, ctx.Request)
	})
	g.Run()
}
