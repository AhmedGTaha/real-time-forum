package main

import (
	"log"
	"net/http"
	"os"

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

	app, err := handlers.NewApp(db)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/", app.HomeHandler)
	mux.HandleFunc("/api/register", app.RegisterHandler)
	mux.HandleFunc("/api/login", app.LoginHandler)
	mux.HandleFunc("/api/me", app.CurrentUserHandler)
	mux.HandleFunc("/api/logout", app.LogoutHandler)
	mux.HandleFunc("/api/posts", app.PostsHandler)
	mux.HandleFunc("/api/comments", app.CommentsHandler)
	mux.HandleFunc("/api/comments/mine", app.MyCommentsHandler)
	mux.HandleFunc("/api/comments/liked", app.LikedCommentsHandler)
	mux.HandleFunc("/api/likes/post", app.TogglePostLikeHandler)
	mux.HandleFunc("/api/likes/comment", app.ToggleCommentLikeHandler)
	mux.HandleFunc("/api/chat/users", app.ChatUsersHandler)
	mux.HandleFunc("/api/chat/messages", app.ChatMessagesHandler)
	mux.HandleFunc("/api/admin/overview", app.AdminOverviewHandler)
	mux.HandleFunc("/api/admin/users", app.AdminUsersHandler)
	mux.HandleFunc("/api/admin/posts", app.AdminPostsHandler)
	mux.HandleFunc("/api/admin/comments", app.AdminCommentsHandler)
	mux.HandleFunc("/ws/chat", app.ChatWebSocketHandler)

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server started on http://localhost:%s", port)

	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal("server error:", err)
	}
}
