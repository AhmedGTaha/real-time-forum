# Real-Time Forum

A single-page forum web application built with a **Go** backend and a **vanilla
JavaScript** frontend. Users can register, publish categorized posts, comment,
like posts and comments, and chat with each other in real time over WebSockets.

The entire frontend is one HTML page driven by JavaScript — navigation, feeds,
the post composer, the profile views, and the chat panel are all rendered
client-side without a page reload.

## Features

- **Authentication** — register with nickname, age, gender, first/last name,
  email, and password. Log in with either your nickname *or* email. Sessions are
  cookie-based and stored server-side with expiry.
- **Posts** — create posts with one or more categories, browse the full feed,
  and filter by search text. Dedicated views for *your* posts and posts you have
  liked.
- **Comments** — comment on any post, plus a view of the comments you have liked.
- **Likes** — toggle likes on both posts and comments; counts update live.
- **Real-time private chat** — one-to-one messaging over a WebSocket connection,
  with online/offline presence and persisted message history.
- **Admin panel** — admin accounts can review every user, post, and comment and
  delete users (and their content), posts, or comments.
- **SQLite storage** — everything persists to a local `forum.db` file. Schema is
  created (and lightly migrated) automatically on startup.

## Tech Stack

| Layer     | Technology                                             |
| --------- | ------------------------------------------------------ |
| Backend   | Go (standard `net/http`)                               |
| Database  | SQLite via [`mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3) |
| Realtime  | [`gorilla/websocket`](https://github.com/gorilla/websocket) |
| Passwords | `golang.org/x/crypto/bcrypt`                           |
| Sessions  | [`google/uuid`](https://github.com/google/uuid)        |
| Frontend  | Vanilla HTML, CSS, and JavaScript (no framework)       |

## Project Structure

```
.
├── main.go                 # Entry point: opens the DB, wires routes, starts the server
├── database/
│   └── db.go               # Connection helper, schema creation, lightweight migrations
├── handlers/               # HTTP + WebSocket handlers, grouped by feature
│   ├── app.go              # App struct holding the DB and the chat Hub
│   ├── auth.go             # Register / login / logout
│   ├── session.go          # Session creation and lookup
│   ├── posts.go            # Post CRUD and feed queries
│   ├── comments.go         # Comment listing and creation
│   ├── likes.go            # Post / comment like toggles
│   ├── user_comments.go    # "My comments" / "liked comments" views
│   ├── chat.go             # Chat REST endpoints (user list, message history)
│   ├── ws.go               # WebSocket upgrade + per-connection read/write loops
│   ├── hub.go              # Central hub tracking connected clients and presence
│   ├── admin.go            # Admin overview and moderation actions
│   ├── home.go             # Serves the SPA shell
│   └── helpers.go          # JSON response helpers
├── static/                 # Frontend served at /static
│   ├── index.html          # Single-page app shell
│   ├── css/style.css
│   └── js/app.js           # All client-side logic
└── cmd/seed/
    └── main.go             # Seeds forum.db with sample data for testing
```

## Getting Started

### Prerequisites

- **Go 1.26+**
- A C compiler (the `go-sqlite3` driver uses cgo — e.g. Xcode command line tools
  on macOS, or `gcc` on Linux).

### Run the server

```bash
git clone <repo-url>
cd real-time-forum
go run .
```

The server starts on **http://localhost:8080**. The database file `forum.db` and
all tables are created automatically on first run.

### Seed sample data (optional)

To populate the forum with sample users, posts, comments, likes, and chat
messages:

```bash
go run ./cmd/seed
```

The seed command is safe to re-run — users are only created if missing, and
sample content is only inserted when the posts table is empty. Delete `forum.db`
first if you want a completely fresh start.

Seeded accounts:

| Account       | Login          | Password      | Role  |
| ------------- | -------------- | ------------- | ----- |
| Admin         | `ahmed`        | `ahmed`       | Admin |
| Regular users | e.g. `alice`   | `password123` | User  |

## API Overview

All endpoints return JSON. Authenticated requests rely on the session cookie set
at login.

| Method | Path                    | Description                          |
| ------ | ----------------------- | ------------------------------------ |
| POST   | `/api/register`         | Create a new account                 |
| POST   | `/api/login`            | Log in (nickname or email)           |
| POST   | `/api/logout`           | End the current session              |
| GET    | `/api/me`               | Current authenticated user           |
| GET/POST | `/api/posts`          | List the feed / create a post        |
| GET/POST | `/api/comments`       | List post comments / add a comment   |
| POST   | `/api/likes/post`       | Toggle a like on a post              |
| POST   | `/api/likes/comment`    | Toggle a like on a comment           |
| GET    | `/api/chat/users`       | Chat contact list with presence      |
| GET    | `/api/chat/messages`    | Message history with another user    |
| WS     | `/ws/chat`              | Real-time chat WebSocket             |

## Data Model

SQLite tables: `users`, `sessions`, `posts`, `categories`, `post_categories`,
`comments`, `post_likes`, `comment_likes`, and `messages`. Foreign keys are
enabled with `ON DELETE CASCADE`, so removing a user or post cleans up its
related rows automatically.
