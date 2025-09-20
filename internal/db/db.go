package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sqlx.DB
}

func New(dbPath string) (*DB, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) RunMigrations(migrationsDir string) error {
	if err := db.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrations, err := getMigrationFiles(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	appliedMigrations, err := db.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	for _, migration := range migrations {
		if _, applied := appliedMigrations[migration.name]; !applied {
			if err := db.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.name, err)
			}
		}
	}

	return nil
}

type migration struct {
	name    string
	version int
	path    string
}

func (db *DB) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.Exec(query)
	return err
}

func getMigrationFiles(dir string) ([]migration, error) {
	var migrations []migration

	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(info.Name(), ".sql") {
			return nil
		}

		parts := strings.SplitN(info.Name(), "_", 2)
		if len(parts) != 2 {
			return nil
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil
		}

		migrations = append(migrations, migration{
			name:    strings.TrimSuffix(info.Name(), ".sql"),
			version: version,
			path:    path,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func (db *DB) getAppliedMigrations() (map[string]bool, error) {
	query := "SELECT name FROM migrations"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}

	return applied, rows.Err()
}

func (db *DB) applyMigration(m migration) error {
	content, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	if _, err := tx.Exec("INSERT INTO migrations (version, name) VALUES (?, ?)", m.version, m.name); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) UpsertFolder(title string, parentID *int) (int64, error) {
	var folderID int64

	err := db.Get(&folderID, "SELECT id FROM folders WHERE title = ?", title)
	if err == sql.ErrNoRows {
		result, err := db.Exec("INSERT INTO folders (title, parent_id) VALUES (?, ?)", title, parentID)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	} else if err != nil {
		return 0, err
	}

	return folderID, nil
}

func (db *DB) UpsertTag(title string) (int64, error) {
	var tagID int64

	err := db.Get(&tagID, "SELECT id FROM tags WHERE title = ?", title)
	if err == sql.ErrNoRows {
		result, err := db.Exec("INSERT INTO tags (title) VALUES (?)", title)
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	} else if err != nil {
		return 0, err
	}

	return tagID, nil
}

func (db *DB) UpdateFolderPaths() error {
	folders := []struct {
		ID       int64  `db:"id"`
		Title    string `db:"title"`
		ParentID *int64 `db:"parent_id"`
	}{}

	if err := db.Select(&folders, "SELECT id, title, parent_id FROM folders ORDER BY id"); err != nil {
		return err
	}

	pathMap := make(map[int64]string)

	var buildPath func(int64) string
	buildPath = func(id int64) string {
		if path, exists := pathMap[id]; exists {
			return path
		}

		for _, folder := range folders {
			if folder.ID == id {
				if folder.ParentID == nil {
					pathMap[id] = folder.Title
					return folder.Title
				}
				parentPath := buildPath(*folder.ParentID)
				fullPath := parentPath + "/" + folder.Title
				pathMap[id] = fullPath
				return fullPath
			}
		}
		return ""
	}

	for _, folder := range folders {
		path := buildPath(folder.ID)
		if _, err := db.Exec("UPDATE folders SET path_cache = ? WHERE id = ?", path, folder.ID); err != nil {
			return err
		}
	}

	return nil
}