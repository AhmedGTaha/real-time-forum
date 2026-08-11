package handlers

import "database/sql"

type App struct {
	DB  *sql.DB
	Hub *Hub
}

// NewApp receives the database connection from main.go and returns an App.
func NewApp(db *sql.DB) (*App, error) {
	hub, err := NewHub()
	if err != nil {
		return nil, err
	}

	return &App{
		DB:  db,
		Hub: hub,
	}, nil
}

func (app *App) Close() error {
	return app.Hub.Close()
}
