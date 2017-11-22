package main

import (
	"net/http"
	"os"

	"github.com/abraithwaite/tools/ws/chatroom"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	r := http.NewServeMux()
	rm := mux.NewRouter()
	b := broadcast.NewWS()
	r.Handle("/", http.FileServer(http.Dir(".")))
	rm.Handle("/chat/{room}", b)
	r.Handle("/chat/", rm)
	h := handlers.LoggingHandler(os.Stderr, r)
	panic(http.ListenAndServe(":8080", h))
}
