package main

import (
	"log"
	"net/http"

	"real-time-forum/database"
	"real-time-forum/handlers"
)

func main() {
	db, err := database.OpenDB("forum.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	err = database.CreateTables(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Database tables ready")

	app := handlers.NewApp(db)

	mux := http.NewServeMux()

	mux.HandleFunc("/", app.HomeHandler)
	mux.HandleFunc("/api/register", app.RegisterHandler)
	mux.HandleFunc("/api/login", app.LoginHandler)
	mux.HandleFunc("/api/me", app.CurrentUserHandler)

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	log.Println("Server started on http://localhost:8080")

	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal("server error:", err)
	}
}
