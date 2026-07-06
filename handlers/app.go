package handlers

import "database/sql"

type App struct {
	DB  *sql.DB
	Hub *Hub
}

// NewApp receives the database connection from main.go and returns an App.
func NewApp(db *sql.DB) *App {
	return &App{
		DB:  db,
		Hub: NewHub(),
	}
}
