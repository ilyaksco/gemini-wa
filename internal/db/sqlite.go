package db

import (
	"context"
	"database/sql"
	"log"
	_ "modernc.org/sqlite"
)

type Database struct {
	*sql.DB
}

type HistoryMessage struct {
	Role    string
	Message string
	UserName string

}

func New(dbPath string) *Database {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connection successful")
	return &Database{db}
}

func (db *Database) InitSchema() {
	userQuery := `
    CREATE TABLE IF NOT EXISTS users (
        jid TEXT PRIMARY KEY,
        lang TEXT NOT NULL DEFAULT 'en'
    );`
	historyQuery := `
    CREATE TABLE IF NOT EXISTS conversation_history (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        jid TEXT NOT NULL,
        role TEXT NOT NULL,
        message TEXT NOT NULL,
        user_name TEXT,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
    );`

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, userQuery); err != nil {
		log.Fatalf("Failed to create users schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, historyQuery); err != nil {
		log.Fatalf("Failed to create history schema: %v", err)
	}

	log.Println("Database schema initialized")
}

func (db *Database) AddMessageToHistory(jid, role, message, userName string) {
	insertQuery := `INSERT INTO conversation_history (jid, role, message, user_name) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(insertQuery, jid, role, message, userName)
	if err != nil {
		log.Printf("Failed to add message to history for %s: %v", jid, err)
		return
	}
}

func (db *Database) GetConversationHistory(jid string) []HistoryMessage {
    query := `
    SELECT role, message, user_name FROM (
        SELECT role, message, user_name, timestamp FROM conversation_history
        WHERE jid = ?
        ORDER BY timestamp DESC
        LIMIT 20
    ) AS recent_messages ORDER BY timestamp ASC;`

    rows, err := db.Query(query, jid)
    if err != nil {
        log.Printf("Failed to get conversation history for %s: %v", jid, err)
        return nil
    }
    defer rows.Close()

    var history []HistoryMessage
    for rows.Next() {
        var h HistoryMessage
        var userName sql.NullString
        if err := rows.Scan(&h.Role, &h.Message, &userName); err != nil {
            log.Printf("Failed to scan history row for %s: %v", jid, err)
            continue
        }
        h.UserName = userName.String
        history = append(history, h)
    }
    return history
}

func (db *Database) DeleteConversationHistory(jid string) error {
	query := `DELETE FROM conversation_history WHERE jid = ?`
	_, err := db.Exec(query, jid)
	if err != nil {
		log.Printf("Failed to delete history for %s: %v", jid, err)
	} else {
		log.Printf("Successfully deleted conversation history for %s", jid)
	}
	return err
}

func (db *Database) GetUserLang(jid string) string {
	var lang string
	query := `SELECT lang FROM users WHERE jid = ?`
	err := db.QueryRow(query, jid).Scan(&lang)
	if err != nil {
		if err == sql.ErrNoRows {
			return "en"
		}
		log.Printf("Failed to get user lang for %s: %v", jid, err)
		return "en"
	}
	return lang
}

func (db *Database) SetUserLang(jid, lang string) error {
	query := `INSERT INTO users (jid, lang) VALUES (?, ?) ON CONFLICT(jid) DO UPDATE SET lang = excluded.lang;`
	_, err := db.Exec(query, jid, lang)
	if err != nil {
		log.Printf("Failed to set user lang for %s: %v", jid, err)
	}
	return err
}