package main

import (
	"log"
	"net/http"

	"real-time-forum/handlers"
)

func main() {
	mux := http.NewServeMux()

	// It decides which handler should respond to each URL
	mux.HandleFunc("/", handlers.HomeHandler)

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	log.Println("Server started on http://localhost:8080")

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal("server error:", err)
	}
}