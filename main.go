package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// type MessageType int
// The message types are defined in RFC 6455, section 11.8.
const (

	// Not in RFC 6455
	ClientConnected = -1

	// Not in RFC 6455
	ClientDisconnected = -2

	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a pong control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

const chat_channel = "chat_channel"

var redis_client *redis.Client
var clients map[string]*websocket.Conn

func init() {
	redis_client = redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "passwd123", // no password set
		DB:       0,           // use default DB
	})
	clients = make(map[string]*websocket.Conn)
}

type Message struct {
	Type int             `json:"type"`
	From string          `json:"from"`
	Conn *websocket.Conn `json:"-"`
	Data string          `json:"data"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func AlphaNumeric(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

func PublishToChannel(typ int, user string, conn *websocket.Conn, msg_data string) {
	new_msg := Message{
		Type: typ,
		From: user,
		Conn: conn,
		Data: msg_data,
	}
	data, err := json.Marshal(new_msg)
	if err != nil {
		log.Print("Marshalling error")
		return
	}
	err = redis_client.Publish(context.Background(), chat_channel, data).Err()
	if err != nil {
		log.Println("could not publish to channel", err)
	}
}

func handleSigleConnection(user string) {
	defer func() {
		PublishToChannel(ClientDisconnected, user, clients[user], "")
		clients[user].Close()
		delete(clients, user)
	}()
	for {
		msg_type, msg, err := clients[user].ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ERROR: %v\n", err)
			}
			return
		}
		PublishToChannel(msg_type, user, clients[user], string(msg))
	}
}

func handleBroadcast() {
	pubsub := redis_client.Subscribe(context.Background(), chat_channel)
	defer pubsub.Close()
	for {
		for new_msg := range pubsub.Channel() {
			msg := Message{}
			err := json.Unmarshal([]byte(new_msg.Payload), &msg)
			if err != nil {
				log.Print("Unmarshalling error")
				return
			}
			for user, c := range clients {
				if msg.From != user {
					c.WriteMessage(websocket.TextMessage, []byte(new_msg.Payload))
				}
			}
		}
	}
}

func main() {

	go handleBroadcast()

	router := gin.Default()
	// v1 := router.Group("/v1")

    router.Static("/static", "./static")
    router.LoadHTMLGlob("./static/*.html")

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"msg": "server online",
		})
	})

	router.GET("/user/:username", func(c *gin.Context) {
		// user := c.Param("username")
		c.HTML(http.StatusOK, "home.html", gin.H{
			"msg": "home page",
		})
	})

	router.GET("/ws/user/:username", func(c *gin.Context) {
        upgrader.CheckOrigin = func(r *http.Request) bool { return true }
        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil {
            c.JSON(500, gin.H{
                "msg":"server error",
            })
        }
		user := c.Param("username")
        PublishToChannel(ClientConnected, user, conn, "")
        clients[user] = conn
        go handleSigleConnection(user)
	})

	exit := make(chan int, 0)
	go func() {
		err := router.Run(":5000")
		if err != nil {
			log.Fatalf("ERROR: listen on port 5000 failed %v", err)
		}
		exit <- 1
	}()
	<-exit
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	redis_client.Shutdown(ctx)
}

func main2() {

	go handleBroadcast()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("server online"))
	})

	// http.Handle("/user/", http.HandlerFunc(serve_user))
	// http.Handle("/ws/user/", http.HandlerFunc(serve_ws_user))

	exit := make(chan int, 0)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/user/static/", http.StripPrefix("/user/static/", fs))
	http_server := http.Server{Addr: ":5000"}

	go func() {
		err := http_server.ListenAndServe()
		if err != nil {
			log.Fatalf("ERROR: listen on port 5000 failed %v", err)
		}
		exit <- 1
	}()
	<-exit
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	redis_client.Shutdown(ctx)
	http_server.Shutdown(ctx)
}
