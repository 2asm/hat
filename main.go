package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	_"github.com/joho/godotenv"
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
		DB:       0,  // use default DB
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
    for _,c := range s{
        if !( (c>='0' && c<='9') || (c>='a' && c<='z') || (c>='A' && c<='Z') ) {
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

func serve_user(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := strings.TrimPrefix(r.URL.Path, "/user/")
    if !AlphaNumeric(user) {
        w.Write([]byte("Not Found"))
        return 
    }
	log.Println(user)
	http.ServeFile(w, r, "static/home.html")
}

func serve_ws_user(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("upgrade") == "" {
		http.Error(w, "Connection not upgraded", http.StatusBadRequest)
		return
	}
	user := strings.TrimPrefix(r.URL.Path, "/ws/user/")
    if !AlphaNumeric(user) {
        w.Write([]byte("Not Found"))
        return
    }
	log.Println(user)
    upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	PublishToChannel(ClientConnected, user, conn, "")
	clients[user] = conn
	go handleSigleConnection(user)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("server online"))
	})

	http.Handle("/user/", http.HandlerFunc(serve_user))
	http.Handle("/ws/user/", http.HandlerFunc(serve_ws_user))

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
