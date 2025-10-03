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

	// Enable foreign keys for this connection
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
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

// UpsertArticleFTS updates the FTS table entry for an article
func (db *DB) UpsertArticleFTS(articleID int64) error {
	// Get article data including tags and folder
	query := `
		SELECT
			a.id, a.url, a.title, a.content_md,
			f.path_cache as folder_path,
			GROUP_CONCAT(t.title, ', ') as tags
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		WHERE a.id = ?
		GROUP BY a.id
	`

	var article struct {
		ID         int64   `db:"id"`
		URL        string  `db:"url"`
		Title      string  `db:"title"`
		ContentMD  *string `db:"content_md"`
		FolderPath *string `db:"folder_path"`
		Tags       *string `db:"tags"`
	}

	if err := db.Get(&article, query, articleID); err != nil {
		return fmt.Errorf("failed to get article data: %w", err)
	}

	// Prepare FTS values
	content := ""
	if article.ContentMD != nil {
		content = *article.ContentMD
	}

	folder := ""
	if article.FolderPath != nil {
		folder = *article.FolderPath
	}

	tags := ""
	if article.Tags != nil {
		tags = *article.Tags
	}

	// Insert or replace in FTS table
	_, err := db.Exec(`
		INSERT OR REPLACE INTO articles_fts (rowid, url, title, content, folder, tags)
		VALUES (?, ?, ?, ?, ?, ?)
	`, articleID, article.URL, article.Title, content, folder, tags)

	if err != nil {
		return fmt.Errorf("failed to update FTS table: %w", err)
	}

	return nil
}

// DeleteArticleFTS removes an article from the FTS table
func (db *DB) DeleteArticleFTS(articleID int64) error {
	_, err := db.Exec("DELETE FROM articles_fts WHERE rowid = ?", articleID)
	if err != nil {
		return fmt.Errorf("failed to delete from FTS table: %w", err)
	}
	return nil
}

// RebuildFTS rebuilds the entire FTS table from scratch
func (db *DB) RebuildFTS() error {
	// For contentless FTS tables, we need to drop and recreate instead of DELETE
	// First, drop the existing FTS table
	if _, err := db.Exec("DROP TABLE IF EXISTS articles_fts"); err != nil {
		return fmt.Errorf("failed to drop FTS table: %w", err)
	}

	// Recreate the FTS table
	if _, err := db.Exec(`CREATE VIRTUAL TABLE articles_fts USING fts5(
		url, title, content, folder, tags, content=''
	)`); err != nil {
		return fmt.Errorf("failed to recreate FTS table: %w", err)
	}

	// Get all article IDs
	var articleIDs []int64
	if err := db.Select(&articleIDs, "SELECT id FROM articles WHERE obsolete = FALSE ORDER BY id"); err != nil {
		return fmt.Errorf("failed to get article IDs: %w", err)
	}

	fmt.Printf("Rebuilding FTS for %d articles...\n", len(articleIDs))

	// Rebuild FTS entries for all articles
	for i, articleID := range articleIDs {
		if err := db.UpsertArticleFTS(articleID); err != nil {
			return fmt.Errorf("failed to rebuild FTS for article %d: %w", articleID, err)
		}

		// Print progress every 1000 articles
		if (i+1)%1000 == 0 {
			fmt.Printf("Rebuilt FTS for %d/%d articles...\n", i+1, len(articleIDs))
		}
	}

	fmt.Printf("Successfully rebuilt FTS for %d articles.\n", len(articleIDs))
	return nil
}

// AddRSSFeed adds a new RSS feed with optional tags
func (db *DB) AddRSSFeed(url, name string, tags []string) (int64, error) {
	// Insert the feed
	result, err := db.Exec(`
		INSERT INTO rss_feeds (url, name)
		VALUES (?, ?)
	`, url, name)
	if err != nil {
		return 0, fmt.Errorf("failed to insert RSS feed: %w", err)
	}

	feedID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get feed ID: %w", err)
	}

	// Add tags if provided
	for _, tagTitle := range tags {
		tagID, err := db.UpsertTag(tagTitle)
		if err != nil {
			return 0, fmt.Errorf("failed to upsert tag: %w", err)
		}

		_, err = db.Exec(`
			INSERT INTO rss_feed_tags (feed_id, tag_id)
			VALUES (?, ?)
		`, feedID, tagID)
		if err != nil {
			return 0, fmt.Errorf("failed to associate tag: %w", err)
		}
	}

	return feedID, nil
}

// GetRSSFeeds retrieves all RSS feeds with their tags
func (db *DB) GetRSSFeeds() ([]map[string]interface{}, error) {
	query := `
		SELECT
			f.id, f.url, f.name, f.created_at, f.last_synced_at, f.active,
			GROUP_CONCAT(t.title, ', ') as tags
		FROM rss_feeds f
		LEFT JOIN rss_feed_tags rft ON f.id = rft.feed_id
		LEFT JOIN tags t ON rft.tag_id = t.id
		GROUP BY f.id
		ORDER BY f.id
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query RSS feeds: %w", err)
	}
	defer rows.Close()

	var feeds []map[string]interface{}
	for rows.Next() {
		var id int64
		var url, name, createdAt string
		var lastSyncedAt, tags *string
		var active bool

		err := rows.Scan(&id, &url, &name, &createdAt, &lastSyncedAt, &active, &tags)
		if err != nil {
			return nil, fmt.Errorf("failed to scan RSS feed: %w", err)
		}

		feed := map[string]interface{}{
			"id":         id,
			"url":        url,
			"name":       name,
			"created_at": createdAt,
			"active":     active,
		}

		if lastSyncedAt != nil {
			feed["last_synced_at"] = *lastSyncedAt
		}

		if tags != nil {
			feed["tags"] = *tags
		}

		feeds = append(feeds, feed)
	}

	return feeds, rows.Err()
}

// GetRSSFeed retrieves a single RSS feed by ID with its tags
func (db *DB) GetRSSFeed(id int64) (map[string]interface{}, []string, error) {
	query := `
		SELECT id, url, name, created_at, last_synced_at, active
		FROM rss_feeds
		WHERE id = ?
	`

	var feedID int64
	var url, name, createdAt string
	var lastSyncedAt *string
	var active bool

	err := db.QueryRow(query, id).Scan(&feedID, &url, &name, &createdAt, &lastSyncedAt, &active)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get RSS feed: %w", err)
	}

	feed := map[string]interface{}{
		"id":         feedID,
		"url":        url,
		"name":       name,
		"created_at": createdAt,
		"active":     active,
	}

	if lastSyncedAt != nil {
		feed["last_synced_at"] = *lastSyncedAt
	}

	// Get tags
	var tags []string
	err = db.Select(&tags, `
		SELECT t.title
		FROM tags t
		JOIN rss_feed_tags rft ON t.id = rft.tag_id
		WHERE rft.feed_id = ?
	`, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("failed to get feed tags: %w", err)
	}

	return feed, tags, nil
}

// DeleteRSSFeed removes an RSS feed and its tag associations
func (db *DB) DeleteRSSFeed(id int64) error {
	result, err := db.Exec("DELETE FROM rss_feeds WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete RSS feed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("RSS feed not found")
	}

	return nil
}

// UpdateRSSFeed updates an RSS feed's name and/or tags
func (db *DB) UpdateRSSFeed(id int64, name *string, tags []string) error {
	// Update name if provided
	if name != nil {
		_, err := db.Exec("UPDATE rss_feeds SET name = ? WHERE id = ?", *name, id)
		if err != nil {
			return fmt.Errorf("failed to update RSS feed name: %w", err)
		}
	}

	// Update tags if provided
	if tags != nil {
		// Remove existing tags
		_, err := db.Exec("DELETE FROM rss_feed_tags WHERE feed_id = ?", id)
		if err != nil {
			return fmt.Errorf("failed to remove existing tags: %w", err)
		}

		// Add new tags
		for _, tagTitle := range tags {
			tagID, err := db.UpsertTag(tagTitle)
			if err != nil {
				return fmt.Errorf("failed to upsert tag: %w", err)
			}

			_, err = db.Exec(`
				INSERT INTO rss_feed_tags (feed_id, tag_id)
				VALUES (?, ?)
			`, id, tagID)
			if err != nil {
				return fmt.Errorf("failed to associate tag: %w", err)
			}
		}
	}

	return nil
}