# websocket

一个封装的 websocket 连接，带有全局消息和独立消息，心跳机制和延时机制，单独处理读写，通过一个调度协程使效率更高
  
## 基本实现概念  

内容并不复杂，全局一个 Hub对象，功能主要有 serve构建，一个 read 协程，一个 write 协程，一个用于调度的 Run 协程

## 1.构建基本 Hub 对象

```go
  type Hub struct {
    // Registered clients.
    clients map[string]*Client

    //Broadcast 公告消息队列
    Broadcast chan Message

    //用户私人消息队列
    BroadcastUser chan Message

    //OnMessage 当收到任意一个客户端发送到消息时触发
    OnMessage OnMessageFunc

    // Register requests from the clients.
    register chan *Client

    // Unregister requests from clients.
    unregister chan *Client
  }
```

保存了所有的连接，以及基本的注册，销毁的通道，几类消息的过渡通道  
并且设定收到消息时的反调函数 OnMessageFunc  

## 2.ServeWs 构建 websocket 连接

```go
  func ServeWs(ctx context.Context, userFlag string, sendCap int, hub *Hub, w http.ResponseWriter, r *http.Request) {
    //生成一个client，里面包含用户信息连接信息等信息
    client := &Client{hub: hub, send: make(chan []byte, sendCap)}
    h := http.Header{}
    if userFlag != "" {
      pro := r.Header.Get(userFlag)
      h.Add(userFlag, pro) //带有websocket的 Protocol子header 需要传入对应header，不然会有1006错误
      client.userFlag = pro
    }
    conn, err := upgrader.Upgrade(w, r, h)
    if err != nil {
      logrus.WithFields(logrus.Fields{"connect or Protocol is nil": err}).Info("websocket")
      return
    }
    client.conn = conn
    client.hub.register <- client //将这个连接放入注册，在run中会加一个
    go client.writePump(ctx)      //新开一个写入，因为有一个用户连接就新开一个，相互不影响，在内部实现心跳包检测连接，详细看函数内部
    client.readPump()             //读取websocket中的信息，详细看函数内部
  }
```

主要获取基本信息，获取 Sec-WebSocket-Protocol 中的用户标识，构建并保存 websocket 对象，同时开启读写协程

## 3.read 和 write 读写对应的消息

一个连接对象，对应一个读和写，所以说，每个连接都会开启新的两个协程，断开后销毁  

```go
  func (c *Client) readPump() {
    defer func() {
      c.hub.unregister <- c //读取完毕后注销该client
      c.conn.Close()
      logrus.Info(c.userFlag + " websocket Close")
    }()
    ......
    for {
      _, message, err := c.conn.ReadMessage()
    ......
      if c.hub.OnMessage != nil { //执行回调函数
        if err := c.hub.OnMessage(message, c.hub); err != nil {
        }
      }
    }
  }
```

**read：**读取消息，并更新死亡时间，读取失败就清除该连接,读取后调用 OnMessage 回调函数  

```go
  func (c *Client) writePump(ctx context.Context) {
    ticker := time.NewTicker(pingPeriod) //设置定时
    defer func() {
      ticker.Stop()
      c.conn.Close()
    }()
    ......
    for {
      select {
      case message, ok := <-c.send: //这里send里面的值时run里面获取的，在这里才开始实际向前台传值
        ......
        if _, err := w.Write(message); err != nil {
          return
        }
        ......
      case <-ticker.C:
        c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //心跳包，下面ping出错就会报错退出，断开这个连接
    }
  }
```

**write：**写入消息，并更新死亡时间，写入失败就关闭该连接，并且加入心跳设定

## 4.Run 调度逻辑方法

全局只有一个 Run 方法，主要是连接的建立、销毁的处理，以及向客户端发送的几类消息的处理调度

```go
  func (h *Hub) Run(ctx context.Context) {
    for {
      select {
      case client := <-h.register: // 客户端有新的连接就加入一个
      ......
      case client := <-h.unregister: // 客户端断开连接，client会进入unregister中，直接在这里获取，删除一个
      ......
      case message := <-h.Broadcast:// 全局广播消息
        data, err := json.Marshal(message)
      ......
      case message := <-h.BroadcastUser: // 用户消息
      ......
      case <-ctx.Done():
        return
      }
    }
  }
```
