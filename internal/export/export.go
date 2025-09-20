package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/model"
	"instapaper-cli/internal/util"

	"gopkg.in/yaml.v3"
)

type Export struct {
	db *db.DB
}

type ExportAllOptions struct {
	Directory       string
	OnlySynced      bool
	IncludeUnsynced bool
	FolderFilter    string
	TagFilter       string
	Since           string
	Until           string
}

func New(database *db.DB) *Export {
	return &Export{db: database}
}

func (e *Export) ExportArticle(id int64, outPath string, stdout bool) error {
	article, err := e.getArticleWithDetails(id)
	if err != nil {
		return fmt.Errorf("failed to get article: %w", err)
	}

	content, err := e.buildMarkdownContent(*article)
	if err != nil {
		return fmt.Errorf("failed to build content: %w", err)
	}

	if stdout {
		fmt.Print(content)
		return nil
	}

	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported article to: %s\n", outPath)
	return nil
}

func (e *Export) ExportAll(opts ExportAllOptions) error {
	articles, err := e.getArticlesForExport(opts)
	if err != nil {
		return fmt.Errorf("failed to get articles: %w", err)
	}

	if len(articles) == 0 {
		fmt.Println("No articles found matching criteria.")
		return nil
	}

	fmt.Printf("Exporting %d articles...\n", len(articles))

	for i, article := range articles {
		if err := e.exportSingleArticle(article, opts.Directory, opts.IncludeUnsynced); err != nil {
			fmt.Printf("Failed to export article %d (%s): %v\n", article.ID, article.Title, err)
			continue
		}

		if (i+1)%10 == 0 {
			fmt.Printf("Exported %d/%d articles...\n", i+1, len(articles))
		}
	}

	fmt.Printf("Export completed: %d articles\n", len(articles))
	return nil
}

func (e *Export) getArticleWithDetails(id int64) (*model.ArticleWithDetails, error) {
	query := `
		SELECT
			a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
			a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
			a.status_text, a.final_url, a.content_md, a.raw_html,
			f.path_cache as folder_path
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		WHERE a.id = ?
	`

	var article model.ArticleWithDetails
	if err := e.db.Get(&article, query, id); err != nil {
		return nil, err
	}

	tags, err := e.getArticleTags(id)
	if err != nil {
		return nil, err
	}
	article.Tags = tags

	return &article, nil
}

func (e *Export) getArticleTags(articleID int64) ([]string, error) {
	query := `
		SELECT t.title
		FROM tags t
		JOIN article_tags at ON t.id = at.tag_id
		WHERE at.article_id = ?
		ORDER BY t.title
	`

	var tags []string
	if err := e.db.Select(&tags, query, articleID); err != nil {
		return nil, err
	}

	return tags, nil
}

func (e *Export) getArticlesForExport(opts ExportAllOptions) ([]model.ArticleWithDetails, error) {
	query := `
		SELECT DISTINCT
			a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
			a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
			a.status_text, a.final_url, a.content_md, a.raw_html,
			f.path_cache as folder_path
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		WHERE 1=1
	`

	var args []interface{}

	if opts.OnlySynced {
		query += " AND a.content_md IS NOT NULL"
	}

	if opts.FolderFilter != "" {
		query += " AND (f.path_cache = ? OR f.title = ?)"
		args = append(args, opts.FolderFilter, opts.FolderFilter)
	}

	if opts.TagFilter != "" {
		query += " AND t.title = ?"
		args = append(args, opts.TagFilter)
	}

	if opts.Since != "" {
		query += " AND a.instapapered_at >= ?"
		args = append(args, opts.Since)
	}

	if opts.Until != "" {
		query += " AND a.instapapered_at <= ?"
		args = append(args, opts.Until)
	}

	query += " ORDER BY a.instapapered_at DESC"

	var articles []model.ArticleWithDetails
	if err := e.db.Select(&articles, query, args...); err != nil {
		return nil, err
	}

	for i := range articles {
		tags, err := e.getArticleTags(articles[i].ID)
		if err != nil {
			return nil, err
		}
		articles[i].Tags = tags
	}

	return articles, nil
}

func (e *Export) exportSingleArticle(article model.ArticleWithDetails, baseDir string, includeUnsynced bool) error {
	content, err := e.buildMarkdownContent(article)
	if err != nil {
		return err
	}

	if article.ContentMD == nil && !includeUnsynced {
		return nil
	}

	folderPath := baseDir
	if article.FolderPath != "" {
		folderPath = filepath.Join(baseDir, article.FolderPath)
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			return fmt.Errorf("failed to create folder: %w", err)
		}
	}

	filename := e.generateFilename(article)
	filePath := filepath.Join(folderPath, filename)

	filePath = e.resolveFilenameCollision(filePath)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (e *Export) buildMarkdownContent(article model.ArticleWithDetails) (string, error) {
	tags := append([]string{"instapaper"}, article.Tags...)

	instapaperedAt, err := time.Parse(time.RFC3339, article.InstapaperedAt)
	if err != nil {
		return "", fmt.Errorf("failed to parse instapapered_at: %w", err)
	}

	frontMatter := model.FrontMatter{
		Title:          article.Title,
		InstapaperedAt: instapaperedAt,
		ExportedAt:     time.Now().UTC(),
		Source:         article.URL,
		Tags:           tags,
	}

	yamlBytes, err := yaml.Marshal(frontMatter)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	var content strings.Builder

	content.WriteString("---\n")
	content.Write(yamlBytes)
	content.WriteString("---\n\n")

	if article.ContentMD != nil && *article.ContentMD != "" {
		content.WriteString(*article.ContentMD)
	} else {
		content.WriteString(fmt.Sprintf("*Article content not yet fetched. Source: %s*\n", article.URL))
	}

	return content.String(), nil
}

func (e *Export) generateFilename(article model.ArticleWithDetails) string {
	filename := util.SafeFilename(article.Title, article.ID, 120)
	return filename + ".md"
}

func (e *Export) resolveFilenameCollision(originalPath string) string {
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return originalPath
	}

	dir := filepath.Dir(originalPath)
	ext := filepath.Ext(originalPath)
	base := strings.TrimSuffix(filepath.Base(originalPath), ext)

	counter := 2
	for {
		newFilename := fmt.Sprintf("%s-%d%s", base, counter, ext)
		newPath := filepath.Join(dir, newFilename)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}

		counter++
		if counter > 100 {
			newFilename = fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
			return filepath.Join(dir, newFilename)
		}
	}
}