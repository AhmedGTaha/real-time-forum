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
	mux.HandleFunc("/api/logout", app.LogoutHandler)
	mux.HandleFunc("/api/posts", app.PostsHandler)
	mux.HandleFunc("/api/comments", app.CommentsHandler)
	mux.HandleFunc("/api/comments/liked", app.LikedCommentsHandler)
	mux.HandleFunc("/api/comments/mine", app.MyCommentsHandler)
	mux.HandleFunc("/api/likes/post", app.TogglePostLikeHandler)
	mux.HandleFunc("/api/likes/comment", app.ToggleCommentLikeHandler)
	mux.HandleFunc("/api/chat/users", app.ChatUsersHandler)
	mux.HandleFunc("/api/chat/messages", app.ChatMessagesHandler)
	mux.HandleFunc("/ws/chat", app.ChatWebSocketHandler)

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	log.Println("Server started on http://localhost:8080")

	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal("server error:", err)
	}
}
