package importer

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/model"
	"instapaper-cli/internal/util"
)

type Importer struct {
	db *db.DB
}

func New(database *db.DB) *Importer {
	return &Importer{db: database}
}

func (i *Importer) ImportCSV(csvPath string) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV headers: %w", err)
	}

	expectedHeaders := []string{"URL", "Title", "Selection", "Folder", "Timestamp", "Tags"}
	if len(headers) != len(expectedHeaders) {
		return fmt.Errorf("unexpected number of CSV columns: got %d, expected %d", len(headers), len(expectedHeaders))
	}

	for idx, header := range headers {
		if header != expectedHeaders[idx] {
			log.Printf("Warning: unexpected header at position %d: got %q, expected %q", idx, header, expectedHeaders[idx])
		}
	}

	var recordCount, skipCount, processedCount int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading CSV record at line %d: %v", recordCount+2, err)
			skipCount++
			continue
		}

		recordCount++

		if len(record) != 6 {
			log.Printf("Skipping malformed record at line %d: expected 6 fields, got %d", recordCount+1, len(record))
			skipCount++
			continue
		}

		csvRecord := model.CSVRecord{
			URL:       record[0],
			Title:     record[1],
			Selection: record[2],
			Folder:    record[3],
			Tags:      record[5],
		}

		timestamp, err := strconv.ParseInt(record[4], 10, 64)
		if err != nil {
			log.Printf("Skipping record with invalid timestamp at line %d: %v", recordCount+1, err)
			skipCount++
			continue
		}
		csvRecord.Timestamp = timestamp

		if err := i.processRecord(csvRecord); err != nil {
			log.Printf("Error processing record at line %d: %v", recordCount+1, err)
			skipCount++
			continue
		}

		processedCount++

		if processedCount%100 == 0 {
			log.Printf("Processed %d records...", processedCount)
		}
	}

	if err := i.db.UpdateFolderPaths(); err != nil {
		log.Printf("Warning: failed to update folder paths: %v", err)
	}

	log.Printf("Import completed: %d total records, %d processed, %d skipped", recordCount, processedCount, skipCount)
	return nil
}

func (i *Importer) processRecord(record model.CSVRecord) error {
	canonicalURL, err := util.CanonicalizeURL(record.URL)
	if err != nil {
		return fmt.Errorf("failed to canonicalize URL %q: %w", record.URL, err)
	}

	var folderID *int64
	if record.Folder != "" {
		id, err := i.db.UpsertFolder(record.Folder, nil)
		if err != nil {
			return fmt.Errorf("failed to upsert folder %q: %w", record.Folder, err)
		}
		folderID = &id
	}

	instapaperedAt := util.UnixToISO8601(record.Timestamp)

	var existingID int64
	err = i.db.Get(&existingID, "SELECT id FROM articles WHERE url = ?", canonicalURL)

	var selection *string
	if record.Selection != "" {
		selection = &record.Selection
	}

	if err == sql.ErrNoRows {
		result, err := i.db.Exec(`
			INSERT INTO articles (url, title, selection, folder_id, instapapered_at)
			VALUES (?, ?, ?, ?, ?)
		`, canonicalURL, record.Title, selection, folderID, instapaperedAt)
		if err != nil {
			return fmt.Errorf("failed to insert article: %w", err)
		}

		articleID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get article ID: %w", err)
		}

		if err := i.processTags(articleID, record.Tags); err != nil {
			return fmt.Errorf("failed to process tags: %w", err)
		}

		// Update FTS table for new article
		if err := i.db.UpsertArticleFTS(articleID); err != nil {
			log.Printf("Warning: failed to update FTS for new article %d: %v", articleID, err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing article: %w", err)
	} else {
		_, err := i.db.Exec(`
			UPDATE articles
			SET title = ?, selection = ?, folder_id = ?, instapapered_at = ?
			WHERE id = ?
		`, record.Title, selection, folderID, instapaperedAt, existingID)
		if err != nil {
			return fmt.Errorf("failed to update article: %w", err)
		}

		if _, err := i.db.Exec("DELETE FROM article_tags WHERE article_id = ?", existingID); err != nil {
			return fmt.Errorf("failed to delete existing tags: %w", err)
		}

		if err := i.processTags(existingID, record.Tags); err != nil {
			return fmt.Errorf("failed to process tags: %w", err)
		}

		// Update FTS table for updated article
		if err := i.db.UpsertArticleFTS(existingID); err != nil {
			log.Printf("Warning: failed to update FTS for updated article %d: %v", existingID, err)
		}
	}

	return nil
}

func (i *Importer) processTags(articleID int64, tagsStr string) error {
	tags := util.ParseTags(tagsStr)
	tags = util.DedupeStrings(tags)

	for _, tagTitle := range tags {
		tagID, err := i.db.UpsertTag(tagTitle)
		if err != nil {
			return fmt.Errorf("failed to upsert tag %q: %w", tagTitle, err)
		}

		_, err = i.db.Exec(`
			INSERT OR IGNORE INTO article_tags (article_id, tag_id)
			VALUES (?, ?)
		`, articleID, tagID)
		if err != nil {
			return fmt.Errorf("failed to link article to tag: %w", err)
		}
	}

	return nil
}