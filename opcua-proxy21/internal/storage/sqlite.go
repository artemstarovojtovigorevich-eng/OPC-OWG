package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const DBPath = "/etc/opcua-proxy.db"

var DB *sql.DB

func Init() error {
	if err := os.MkdirAll(filepath.Dir(DBPath), 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	_, err = db.Exec("PRAGMA busy_timeout = 5000")
	if err != nil {
		return fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		return fmt.Errorf("failed to set journal_mode: %w", err)
	}

	if err := createTables(db); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	DB = db
	return nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		data_type TEXT,
		enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS app_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err := db.Exec(schema)
	return err
}

func GetSetting(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func SetSetting(key, value string) error {
	_, err := DB.Exec(`
		INSERT OR REPLACE INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, key, value)
	return err
}

func GetAllNodes() ([]Node, error) {
	rows, err := DB.Query("SELECT id, node_id, name, data_type, enabled FROM nodes ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.NodeID, &n.Name, &n.DataType, &n.Enabled); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func GetEnabledNodes() ([]Node, error) {
	rows, err := DB.Query("SELECT id, node_id, name, data_type, enabled FROM nodes WHERE enabled = 1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.NodeID, &n.Name, &n.DataType, &n.Enabled); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func SaveNode(nodeID, name, dataType string) (int64, error) {
	result, err := DB.Exec(`
		INSERT OR REPLACE INTO nodes (node_id, name, data_type, enabled, created_at)
		VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, nodeID, name, dataType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func SaveNodes(nodes []Node) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, n := range nodes {
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO nodes (node_id, name, data_type, enabled, created_at)
			VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)
		`, n.NodeID, n.Name, n.DataType)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func SetNodeEnabled(id int64, enabled bool) error {
	_, err := DB.Exec("UPDATE nodes SET enabled = ? WHERE id = ?", enabled, id)
	return err
}

func DeleteNode(id int64) error {
	_, err := DB.Exec("DELETE FROM nodes WHERE id = ?", id)
	return err
}

func GetAppState(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM app_state WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func SetAppState(key, value string) error {
	_, err := DB.Exec(`
		INSERT OR REPLACE INTO app_state (key, value)
		VALUES (?, ?)
	`, key, value)
	return err
}

func NodeCount() (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&count)
	return count, err
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

type Node struct {
	ID       int64
	NodeID   string
	Name     string
	DataType string
	Enabled  bool
}