// Command seed populates forum.db with realistic testing data:
// a handful of users, categories, posts, comments, likes, and chat messages.
//
// Run it from the project root:
//
//	go run ./cmd/seed
//
// All seeded users share the password "password123" so you can log in as any
// of them (for example nickname "alice"). Presence/online status still depends
// on the websocket connection, so users only appear "Online" while their
// browser tab is open.
//
// The command is safe to re-run: users are created only if missing, and the
// posts/comments/likes/messages are only inserted when the posts table is
// empty. Delete forum.db first if you want a completely fresh seed.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"real-time-forum/database"

	"golang.org/x/crypto/bcrypt"
)

// seedPassword is shared by every seeded (non-admin) account for easy testing.
const seedPassword = "password123"

// Admin account credentials.
const (
	adminNickname = "ahmed"
	adminPassword = "ahmed"
)

type seedUser struct {
	Nickname  string
	Age       int
	Gender    string
	FirstName string
	LastName  string
	Email     string
}

func main() {
	db, err := database.OpenDB("forum.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Make sure the schema exists before inserting anything.
	if err := database.CreateTables(db); err != nil {
		log.Fatal(err)
	}

	users := []seedUser{
		{"alice", 28, "female", "Alice", "Martin", "alice@example.com"},
		{"bob", 34, "male", "Bob", "Lee", "bob@example.com"},
		{"carol", 25, "female", "Carol", "Diaz", "carol@example.com"},
		{"dave", 41, "male", "Dave", "Kim", "dave@example.com"},
		{"erin", 30, "female", "Erin", "Walsh", "erin@example.com"},
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	// userID maps nickname -> database id so later inserts can reference authors.
	userID := map[string]int64{}
	for _, u := range users {
		id, err := getOrCreateUser(db, u, string(passwordHash))
		if err != nil {
			log.Fatalf("seed user %q: %v", u.Nickname, err)
		}
		userID[u.Nickname] = id
	}

	log.Printf("ensured %d users (password for all: %q)", len(users), seedPassword)

	// Seed the admin account. Its password is intentionally different.
	if err := ensureAdmin(db); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	log.Printf("ensured admin account (nickname: %q, password: %q)", adminNickname, adminPassword)

	// Only seed content once, so re-running does not pile up duplicate posts.
	var postCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM posts;`).Scan(&postCount); err != nil {
		log.Fatal(err)
	}
	if postCount > 0 {
		log.Printf("posts already exist (%d), skipping content seed", postCount)
		log.Println("delete forum.db and re-run to reseed from scratch")
		return
	}

	if err := seedContent(db, userID); err != nil {
		log.Fatal(err)
	}

	log.Println("seed complete: posts, comments, likes, and messages inserted")
}

// getOrCreateUser inserts a user if the nickname is not taken yet, and returns
// the user's id either way.
func getOrCreateUser(db *sql.DB, u seedUser, passwordHash string) (int64, error) {
	var id int64
	err := db.QueryRow(`SELECT id FROM users WHERE nickname = ?;`, u.Nickname).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := db.Exec(`
		INSERT INTO users (nickname, age, gender, first_name, last_name, email, password_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`, u.Nickname, u.Age, u.Gender, u.FirstName, u.LastName, u.Email, passwordHash)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// ensureAdmin creates the admin account if it does not exist, or promotes and
// resets the password of an existing account with the same nickname.
func ensureAdmin(db *sql.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	var id int64
	err = db.QueryRow(`SELECT id FROM users WHERE nickname = ?;`, adminNickname).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`
			INSERT INTO users (nickname, age, gender, first_name, last_name, email, password_hash, is_admin)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1);
		`, adminNickname, 30, "male", "Ahmed", "Admin", "ahmed@example.com", string(hash))
		return err
	}
	if err != nil {
		return err
	}

	// Already exists: make sure it is an admin with the expected password.
	_, err = db.Exec(`
		UPDATE users SET is_admin = 1, password_hash = ? WHERE id = ?;
	`, string(hash), id)
	return err
}

// seedContent inserts posts (with categories), comments, likes, and chat
// messages inside a single transaction so it is all-or-nothing.
func seedContent(db *sql.DB, userID map[string]int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	// ago returns a SQLite datetime string N hours before now.
	ago := func(hours int) string {
		return now.Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05")
	}

	categoryID := map[string]int64{}
	getCategory := func(name string) (int64, error) {
		if id, ok := categoryID[name]; ok {
			return id, nil
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO categories (name) VALUES (?);`, name); err != nil {
			return 0, err
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM categories WHERE name = ?;`, name).Scan(&id); err != nil {
			return 0, err
		}
		categoryID[name] = id
		return id, nil
	}

	type postSpec struct {
		author     string
		title      string
		content    string
		categories []string
		agoHours   int
	}

	posts := []postSpec{
		{"alice", "Welcome to the forum",
			"Hi everyone! This is a small real-time forum built with Go and vanilla JavaScript. Post something, leave a comment, and try the live chat.",
			[]string{"general"}, 96},
		{"bob", "How SQLite handles concurrent writes",
			"SQLite serializes writes with a single writer lock. For a project this size it is more than enough, and WAL mode makes reads even smoother. Anyone using a different setup?",
			[]string{"go", "sqlite"}, 72},
		{"carol", "Building the chat UI",
			"The trickiest part of the chat panel was loading older messages without the scroll position jumping around. Keeping the scroll offset before prepending did the trick.",
			[]string{"frontend", "websockets"}, 48},
		{"dave", "Session cookies vs JWT",
			"For a server-rendered app I still prefer plain session cookies backed by a sessions table. Easy to revoke, no token bloat. JWTs shine more for stateless APIs.",
			[]string{"go", "general"}, 24},
		{"erin", "Dark mode is finally here",
			"Toggle the theme button in the header. The preference is saved in localStorage so it sticks between visits. Let me know if any panel looks off in dark mode.",
			[]string{"frontend"}, 6},
		{"alice", "Show me your best Go tips",
			"Drop your favorite small Go tip below. I'll start: errors.Is/As beats string matching on error text every time.",
			[]string{"go"}, 2},
	}

	// postID collects inserted post ids so comments and likes can target them.
	postID := make([]int64, len(posts))
	for i, p := range posts {
		res, err := tx.Exec(`
			INSERT INTO posts (user_id, title, content, created_at)
			VALUES (?, ?, ?, ?);
		`, userID[p.author], p.title, p.content, ago(p.agoHours))
		if err != nil {
			return fmt.Errorf("insert post %q: %w", p.title, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		postID[i] = id

		for _, c := range p.categories {
			catID, err := getCategory(c)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`
				INSERT OR IGNORE INTO post_categories (post_id, category_id)
				VALUES (?, ?);
			`, id, catID); err != nil {
				return err
			}
		}
	}

	type commentSpec struct {
		postIndex int
		author    string
		content   string
		agoHours  int
	}

	comments := []commentSpec{
		{0, "bob", "Nice, the live chat feels snappy. Great work!", 95},
		{0, "carol", "Love how clean the UI is.", 90},
		{1, "dave", "WAL mode is the default I reach for too. Big difference under load.", 70},
		{1, "alice", "Good reminder to enable foreign keys per connection as well.", 68},
		{2, "erin", "The scroll-anchoring trick is underrated. Thanks for writing it up.", 47},
		{3, "bob", "Agreed. Revocable sessions saved me during an incident once.", 22},
		{4, "carol", "Dark mode looks great on my screen.", 5},
		{5, "dave", "Prefer table-driven tests for anything with more than two cases.", 1},
	}

	// commentID lets us attach comment likes to specific comments.
	commentID := make([]int64, len(comments))
	for i, c := range comments {
		res, err := tx.Exec(`
			INSERT INTO comments (post_id, user_id, content, created_at)
			VALUES (?, ?, ?, ?);
		`, postID[c.postIndex], userID[c.author], c.content, ago(c.agoHours))
		if err != nil {
			return fmt.Errorf("insert comment: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		commentID[i] = id
	}

	// Post likes: (postIndex, liker nickname).
	postLikes := []struct {
		postIndex int
		liker     string
	}{
		{0, "bob"}, {0, "carol"}, {0, "dave"}, {0, "erin"},
		{1, "alice"}, {1, "dave"},
		{2, "alice"}, {2, "bob"}, {2, "erin"},
		{3, "carol"},
		{4, "alice"}, {4, "bob"}, {4, "carol"}, {4, "dave"},
		{5, "erin"}, {5, "bob"},
	}
	for _, l := range postLikes {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO post_likes (post_id, user_id)
			VALUES (?, ?);
		`, postID[l.postIndex], userID[l.liker]); err != nil {
			return err
		}
	}

	// Comment likes: (commentIndex, liker nickname).
	commentLikes := []struct {
		commentIndex int
		liker        string
	}{
		{0, "alice"}, {0, "carol"},
		{2, "bob"},
		{4, "carol"}, {4, "alice"},
		{7, "erin"},
	}
	for _, l := range commentLikes {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO comment_likes (comment_id, user_id)
			VALUES (?, ?);
		`, commentID[l.commentIndex], userID[l.liker]); err != nil {
			return err
		}
	}

	// A couple of chat conversations so the messages panel is not empty.
	type messageSpec struct {
		from     string
		to       string
		content  string
		agoHours int
	}
	messages := []messageSpec{
		{"alice", "bob", "Hey Bob, did you get the SQLite post working?", 71},
		{"bob", "alice", "Yep! WAL mode fixed the locking I was seeing.", 70},
		{"alice", "bob", "Nice. Want to pair on the chat pagination next?", 69},
		{"bob", "alice", "Sure, ping me tomorrow morning.", 68},
		{"carol", "dave", "Dave, your session cookie writeup was really clear.", 23},
		{"dave", "carol", "Thanks Carol! Happy to review the chat UI PR if you want.", 22},
		{"carol", "dave", "That would be great, sending it over shortly.", 21},
	}
	for _, m := range messages {
		if _, err := tx.Exec(`
			INSERT INTO messages (sender_id, receiver_id, content, created_at)
			VALUES (?, ?, ?, ?);
		`, userID[m.from], userID[m.to], m.content, ago(m.agoHours)); err != nil {
			return err
		}
	}

	return tx.Commit()
}
