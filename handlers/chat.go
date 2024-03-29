package handlers

import (
	"bytes"
	"chatting-app/components"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	username string
	conn     *websocket.Conn
}

type ChatHandler struct {
	msgs      *[]components.Message
	upgrader  websocket.Upgrader
	clients   map[*Client]bool
	broadcast chan components.Message
}

func NewChatHandler(msgs *[]components.Message) *ChatHandler {
	h := &ChatHandler{
		msgs:      msgs,
		upgrader:  websocket.Upgrader{},
		clients:   make(map[*Client]bool),
		broadcast: make(chan components.Message),
	}

	go func() {
		for {
			msg := <-h.broadcast

			for client := range h.clients {
				ctx := context.WithValue(context.Background(), "username", client.username)

				var html bytes.Buffer
				components.
					MessagesList([]components.Message{msg}, true).
					Render(ctx, &html)

				if err := client.conn.WriteMessage(websocket.TextMessage, html.Bytes()); err != nil {
					fmt.Println(err)
				}
			}
		}
	}()

	return h
}

func (h *ChatHandler) Page(w http.ResponseWriter, r *http.Request) {
	username := ""
	if cookie, err := r.Cookie("username"); err == nil {
		username = cookie.Value
	}

	ctx := context.WithValue(r.Context(), "username", username)
	components.Page(*h.msgs).Render(ctx, w)
}

func (h *ChatHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Println(err)
	}

	cookie := http.Cookie{
		Name:   "username",
		Value:  r.PostForm.Get("username"),
		Path:   "/",
		MaxAge: 3600,
	}

	http.SetCookie(w, &cookie)
}

func (h *ChatHandler) Chatroom(w http.ResponseWriter, r *http.Request) {
	var username string
	cookie, err := r.Cookie("username")
	if err != nil {
		fmt.Println(err)
		return
	}
	username = cookie.Value

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
	}

	client := &Client{username, conn}
	h.clients[client] = true

	for {
		var payload struct {
			Msg string `json:"msg"`
		}

		_, p, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			break
		}

		err = json.Unmarshal(p, &payload)
		if err != nil {
			fmt.Println(err)
			continue
		}

		msg := components.Message{
			Author:  username,
			Content: payload.Msg,
		}

		*h.msgs = append(*h.msgs, msg)
		h.broadcast <- msg
	}

	delete(h.clients, client)
}
