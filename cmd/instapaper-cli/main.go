package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/export"
	"instapaper-cli/internal/fetcher"
	"instapaper-cli/internal/importer"
	"instapaper-cli/internal/mcp"
	"instapaper-cli/internal/search"

	"github.com/spf13/cobra"
)

var (
	dbPath         string
	migrationsPath string
	database       *db.DB
)

func init() {
	cobra.OnInitialize(initDB)
}

func initDB() {
	if dbPath == "" {
		dbPath = "instapaper.sqlite"
	}

	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	var err error
	database, err = db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if err := database.RunMigrations(migrationsPath); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "instapaper-cli",
		Short: "A CLI tool for managing Instapaper exports",
		Long:  "Import, fetch, search, and export Instapaper articles from CSV exports",
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "instapaper.sqlite", "Path to SQLite database file")
	rootCmd.PersistentFlags().StringVar(&migrationsPath, "migrations", "migrations", "Path to migrations directory")

	var importCmd = &cobra.Command{
		Use:   "import",
		Short: "Import articles from Instapaper CSV export",
		RunE:  runImport,
	}

	var csvPath string
	importCmd.Flags().StringVar(&csvPath, "csv", "", "Path to CSV file (required)")
	importCmd.MarkFlagRequired("csv")

	var fetchCmd = &cobra.Command{
		Use:   "fetch",
		Short: "Fetch article content using readability",
		RunE:  runFetch,
	}

	var (
		fetchOrder              string
		fetchSearch             string
		fetchLimit              int
		fetchPreferExtracted    bool
		fetchStoreRaw          bool
		fetchLogPath           string
	)

	fetchCmd.Flags().StringVar(&fetchOrder, "order", "oldest", "Order articles by 'oldest' or 'newest'")
	fetchCmd.Flags().StringVar(&fetchSearch, "search", "", "Search phrase to filter articles")
	fetchCmd.Flags().IntVar(&fetchLimit, "limit", 10, "Maximum number of articles to fetch")
	fetchCmd.Flags().BoolVar(&fetchPreferExtracted, "prefer-extracted-title", false, "Use extracted title instead of CSV title")
	fetchCmd.Flags().BoolVar(&fetchStoreRaw, "store-raw", false, "Store raw HTML alongside Markdown")
	fetchCmd.Flags().StringVar(&fetchLogPath, "log", "", "Path to log file")

	var searchCmd = &cobra.Command{
		Use:   "search [query]",
		Short: "Search articles",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSearch,
	}

	var (
		searchField string
		searchFTS   bool
		searchLimit int
		searchJSON  bool
		searchSince string
		searchUntil string
	)

	searchCmd.Flags().StringVar(&searchField, "field", "", "Search specific field: url, title, content, tags, folder")
	searchCmd.Flags().BoolVar(&searchFTS, "fts", false, "Use full-text search")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Output results as JSON")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "Filter articles since date (1d, 1w, today, yesterday, 2006-01-02)")
	searchCmd.Flags().StringVar(&searchUntil, "until", "", "Filter articles until date (1d, 1w, today, yesterday, 2006-01-02)")

	var latestCmd = &cobra.Command{
		Use:   "latest",
		Short: "Get latest articles with optional date filtering",
		Long:  "Retrieve the most recent articles, optionally filtered by date range. Useful for finding articles added within specific timeframes.",
		RunE:  runLatest,
	}

	var (
		latestLimit int
		latestJSON  bool
		latestSince string
		latestUntil string
	)

	latestCmd.Flags().IntVar(&latestLimit, "limit", 20, "Maximum number of articles to show")
	latestCmd.Flags().BoolVar(&latestJSON, "json", false, "Output results as JSON")
	latestCmd.Flags().StringVar(&latestSince, "since", "", "Show articles since date (1d, 1w, today, yesterday, 2006-01-02)")
	latestCmd.Flags().StringVar(&latestUntil, "until", "", "Show articles until date (1d, 1w, today, yesterday, 2006-01-02)")

	var exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export a single article",
		RunE:  runExport,
	}

	var (
		exportID     int64
		exportOut    string
		exportStdout bool
	)

	exportCmd.Flags().Int64Var(&exportID, "id", 0, "Article ID to export (required)")
	exportCmd.Flags().StringVar(&exportOut, "out", "", "Output file path")
	exportCmd.Flags().BoolVar(&exportStdout, "stdout", false, "Output to stdout")
	exportCmd.MarkFlagRequired("id")

	var exportAllCmd = &cobra.Command{
		Use:   "export-all",
		Short: "Export all articles to directory",
		RunE:  runExportAll,
	}

	var (
		exportAllDir           string
		exportAllOnlySynced    bool
		exportAllIncludeUnsynced bool
		exportAllFolder        string
		exportAllTag           string
		exportAllSince         string
		exportAllUntil         string
		exportAllFromSearch    string
		exportAllSearchField   string
		exportAllSearchFTS     bool
		exportAllSearchLimit   int
	)

	exportAllCmd.Flags().StringVar(&exportAllDir, "dir", "", "Output directory (required)")
	exportAllCmd.Flags().BoolVar(&exportAllOnlySynced, "only-synced", true, "Only export synced articles")
	exportAllCmd.Flags().BoolVar(&exportAllIncludeUnsynced, "include-unsynced", false, "Include unsynced articles as stubs")
	exportAllCmd.Flags().StringVar(&exportAllFolder, "folder", "", "Filter by folder path")
	exportAllCmd.Flags().StringVar(&exportAllTag, "tag", "", "Filter by tag")
	exportAllCmd.Flags().StringVar(&exportAllSince, "since", "", "Filter articles since date (ISO8601)")
	exportAllCmd.Flags().StringVar(&exportAllUntil, "until", "", "Filter articles until date (ISO8601)")
	exportAllCmd.Flags().StringVar(&exportAllFromSearch, "from-search", "", "Export articles from search results")
	exportAllCmd.Flags().StringVar(&exportAllSearchField, "field", "", "Search specific field: url, title, content, tags, folder")
	exportAllCmd.Flags().BoolVar(&exportAllSearchFTS, "fts", false, "Use full-text search")
	exportAllCmd.Flags().IntVar(&exportAllSearchLimit, "limit", 0, "Maximum number of search results to export")
	exportAllCmd.MarkFlagRequired("dir")

	var foldersCmd = &cobra.Command{
		Use:   "folders",
		Short: "Manage folder hierarchy",
		RunE:  runFolders,
	}

	var (
		foldersAction string
		foldersSource string
		foldersTarget string
		foldersName   string
	)

	foldersCmd.Flags().StringVar(&foldersAction, "action", "list", "Action: list, mv, mkdir")
	foldersCmd.Flags().StringVar(&foldersSource, "source", "", "Source folder for mv")
	foldersCmd.Flags().StringVar(&foldersTarget, "target", "", "Target folder for mv")
	foldersCmd.Flags().StringVar(&foldersName, "name", "", "Folder name for mkdir")

	var tagsCmd = &cobra.Command{
		Use:   "tags",
		Short: "Manage tags",
		RunE:  runTags,
	}

	var (
		tagsAction string
		tagsOld    string
		tagsNew    string
	)

	tagsCmd.Flags().StringVar(&tagsAction, "action", "list", "Action: list, rename")
	tagsCmd.Flags().StringVar(&tagsOld, "old", "", "Old tag name for rename")
	tagsCmd.Flags().StringVar(&tagsNew, "new", "", "New tag name for rename")

	var doctorCmd = &cobra.Command{
		Use:   "doctor",
		Short: "Database integrity checks and maintenance",
		RunE:  runDoctor,
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("instapaper-cli v1.2.1")
		},
	}

	var mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP (Model Context Protocol) server",
		Long:  "Start an MCP server that exposes search and export functionality for AI models and other MCP clients",
		RunE:  runMCP,
	}

	var obsoleteCmd = &cobra.Command{
		Use:   "obsolete",
		Short: "Mark articles as obsolete to exclude from searches and exports",
		Long:  "Mark articles as obsolete based on ID or criteria like status codes and failure counts. Obsolete articles remain in database but are excluded from searches, exports, and fetch attempts.",
		RunE:  runObsolete,
	}

	var (
		obsoleteIDs         []int64
		obsoleteStatusCodes []int
		obsoleteFailureMin  int
		obsoleteDryRun      bool
		obsoleteConfirm     bool
	)

	obsoleteCmd.Flags().Int64SliceVar(&obsoleteIDs, "ids", nil, "Comma-separated list of article IDs to mark obsolete")
	obsoleteCmd.Flags().IntSliceVar(&obsoleteStatusCodes, "status-codes", nil, "Mark articles with these HTTP status codes as obsolete (e.g., 404,403)")
	obsoleteCmd.Flags().IntVar(&obsoleteFailureMin, "min-failures", 0, "Mark articles with at least this many fetch failures as obsolete")
	obsoleteCmd.Flags().BoolVar(&obsoleteDryRun, "dry-run", false, "Show what would be marked obsolete without making changes")
	obsoleteCmd.Flags().BoolVar(&obsoleteConfirm, "confirm", false, "Confirm the operation (required for non-dry-run)")

	var listObsoleteCmd = &cobra.Command{
		Use:   "list-obsolete",
		Short: "List articles marked as obsolete",
		Long:  "List all articles that have been marked as obsolete for management and review",
		RunE:  runListObsolete,
	}

	var (
		listObsoleteJSON  bool
		listObsoleteLimit int
	)

	listObsoleteCmd.Flags().BoolVar(&listObsoleteJSON, "json", false, "Output results as JSON")
	listObsoleteCmd.Flags().IntVar(&listObsoleteLimit, "limit", 100, "Maximum number of obsolete articles to show")

	var statsCmd = &cobra.Command{
		Use:   "stats",
		Short: "Show database statistics and health overview",
		Long:  "Display comprehensive statistics about articles including total count, fetch status, failures, and obsolete articles",
		RunE:  runStats,
	}

	var statsJSON bool
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output statistics as JSON")

	rootCmd.AddCommand(importCmd, fetchCmd, searchCmd, latestCmd, exportCmd, exportAllCmd, foldersCmd, tagsCmd, doctorCmd, versionCmd, mcpCmd, obsoleteCmd, listObsoleteCmd, statsCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}

	if database != nil {
		database.Close()
	}
}

func runImport(cmd *cobra.Command, args []string) error {
	csvPath, _ := cmd.Flags().GetString("csv")

	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		fmt.Printf("CSV file does not exist: %s\n", csvPath)
		return fmt.Errorf("CSV file does not exist: %s", csvPath)
	}

	imp := importer.New(database)
	return imp.ImportCSV(csvPath)
}

func runFetch(cmd *cobra.Command, args []string) error {
	order, _ := cmd.Flags().GetString("order")
	searchPhrase, _ := cmd.Flags().GetString("search")
	limit, _ := cmd.Flags().GetInt("limit")
	preferExtracted, _ := cmd.Flags().GetBool("prefer-extracted-title")
	storeRaw, _ := cmd.Flags().GetBool("store-raw")
	logPath, _ := cmd.Flags().GetString("log")

	opts := fetcher.FetchOptions{
		Order:            order,
		SearchPhrase:     searchPhrase,
		Limit:            limit,
		PreferExtracted:  preferExtracted,
		StoreRaw:         storeRaw,
		LogPath:          logPath,
	}

	f := fetcher.New(database)
	return f.FetchArticles(opts)
}

func runSearch(cmd *cobra.Command, args []string) error {
	var query string
	if len(args) > 0 {
		query = args[0]
	}

	field, _ := cmd.Flags().GetString("field")
	useFTS, _ := cmd.Flags().GetBool("fts")
	limit, _ := cmd.Flags().GetInt("limit")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")

	opts := search.SearchOptions{
		Query:      query,
		Field:      field,
		UseFTS:     useFTS,
		Limit:      limit,
		JSONOutput: jsonOutput,
		Since:      since,
		Until:      until,
	}

	s := search.New(database)
	return s.Search(opts)
}

func runLatest(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")

	// Use search functionality with empty query to get all articles
	opts := search.SearchOptions{
		Query:      "",
		Field:      "",
		UseFTS:     false,
		Limit:      limit,
		JSONOutput: jsonOutput,
		Since:      since,
		Until:      until,
	}

	s := search.New(database)
	return s.Search(opts)
}

func runExport(cmd *cobra.Command, args []string) error {
	id, _ := cmd.Flags().GetInt64("id")
	outPath, _ := cmd.Flags().GetString("out")
	stdout, _ := cmd.Flags().GetBool("stdout")

	if !stdout && outPath == "" {
		return fmt.Errorf("either --out or --stdout must be specified")
	}

	e := export.New(database)
	return e.ExportArticle(id, outPath, stdout)
}

func runExportAll(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	onlySynced, _ := cmd.Flags().GetBool("only-synced")
	includeUnsynced, _ := cmd.Flags().GetBool("include-unsynced")
	folder, _ := cmd.Flags().GetString("folder")
	tag, _ := cmd.Flags().GetString("tag")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	fromSearch, _ := cmd.Flags().GetString("from-search")
	searchField, _ := cmd.Flags().GetString("field")
	searchFTS, _ := cmd.Flags().GetBool("fts")
	searchLimit, _ := cmd.Flags().GetInt("limit")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	opts := export.ExportAllOptions{
		Directory:       dir,
		OnlySynced:      onlySynced && !includeUnsynced,
		IncludeUnsynced: includeUnsynced,
		FolderFilter:    folder,
		TagFilter:       tag,
		Since:           since,
		Until:           until,
		FromSearch:      fromSearch,
		SearchField:     searchField,
		SearchFTS:       searchFTS,
		SearchLimit:     searchLimit,
	}

	e := export.New(database)
	return e.ExportAll(opts)
}

func runFolders(cmd *cobra.Command, args []string) error {
	action, _ := cmd.Flags().GetString("action")

	switch action {
	case "list":
		return listFolders()
	case "mv":
		source, _ := cmd.Flags().GetString("source")
		target, _ := cmd.Flags().GetString("target")
		if source == "" || target == "" {
			return fmt.Errorf("both --source and --target are required for mv action")
		}
		return moveFolders(source, target)
	case "mkdir":
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required for mkdir action")
		}
		return createFolder(name)
	default:
		return fmt.Errorf("invalid action: %s. Use list, mv, or mkdir", action)
	}
}

func runTags(cmd *cobra.Command, args []string) error {
	action, _ := cmd.Flags().GetString("action")

	switch action {
	case "list":
		return listTags()
	case "rename":
		old, _ := cmd.Flags().GetString("old")
		new, _ := cmd.Flags().GetString("new")
		if old == "" || new == "" {
			return fmt.Errorf("both --old and --new are required for rename action")
		}
		return renameTag(old, new)
	default:
		return fmt.Errorf("invalid action: %s. Use list or rename", action)
	}
}

func runDoctor(cmd *cobra.Command, args []string) error {
	return runDatabaseDoctor()
}

func listFolders() error {
	query := `
		SELECT id, title, parent_id, path_cache
		FROM folders
		ORDER BY path_cache
	`

	var folders []struct {
		ID        int64   `db:"id"`
		Title     string  `db:"title"`
		ParentID  *int64  `db:"parent_id"`
		PathCache *string `db:"path_cache"`
	}

	if err := database.Select(&folders, query); err != nil {
		return fmt.Errorf("failed to get folders: %w", err)
	}

	fmt.Printf("%-5s %-30s %-10s %s\n", "ID", "PATH", "PARENT", "TITLE")
	fmt.Println(strings.Repeat("-", 80))

	for _, folder := range folders {
		parentStr := ""
		if folder.ParentID != nil {
			parentStr = fmt.Sprintf("%d", *folder.ParentID)
		}

		pathStr := folder.Title
		if folder.PathCache != nil {
			pathStr = *folder.PathCache
		}

		fmt.Printf("%-5d %-30s %-10s %s\n", folder.ID, pathStr, parentStr, folder.Title)
	}

	return nil
}

func moveFolders(source, target string) error {
	return fmt.Errorf("folder move not yet implemented")
}

func createFolder(name string) error {
	_, err := database.UpsertFolder(name, nil)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	if err := database.UpdateFolderPaths(); err != nil {
		return fmt.Errorf("failed to update folder paths: %w", err)
	}

	fmt.Printf("Created folder: %s\n", name)
	return nil
}

func listTags() error {
	query := `
		SELECT t.id, t.title, COUNT(at.article_id) as article_count
		FROM tags t
		LEFT JOIN article_tags at ON t.id = at.tag_id
		GROUP BY t.id, t.title
		ORDER BY t.title
	`

	var tags []struct {
		ID           int64  `db:"id"`
		Title        string `db:"title"`
		ArticleCount int    `db:"article_count"`
	}

	if err := database.Select(&tags, query); err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	fmt.Printf("%-5s %-30s %s\n", "ID", "TAG", "ARTICLES")
	fmt.Println(strings.Repeat("-", 50))

	for _, tag := range tags {
		fmt.Printf("%-5d %-30s %d\n", tag.ID, tag.Title, tag.ArticleCount)
	}

	return nil
}

func renameTag(old, new string) error {
	result, err := database.Exec("UPDATE tags SET title = ? WHERE title = ?", new, old)
	if err != nil {
		return fmt.Errorf("failed to rename tag: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tag '%s' not found", old)
	}

	fmt.Printf("Renamed tag '%s' to '%s'\n", old, new)
	return nil
}

func runDatabaseDoctor() error {
	fmt.Println("Running database integrity checks...")

	if _, err := database.Exec("PRAGMA integrity_check"); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}

	if _, err := database.Exec("PRAGMA foreign_key_check"); err != nil {
		return fmt.Errorf("foreign key check failed: %w", err)
	}

	var counts struct {
		Articles       int `db:"articles"`
		Folders        int `db:"folders"`
		Tags           int `db:"tags"`
		SyncedArticles int `db:"synced_articles"`
		FailedArticles int `db:"failed_articles"`
	}

	countQuery := `
		SELECT
			(SELECT COUNT(*) FROM articles) as articles,
			(SELECT COUNT(*) FROM folders) as folders,
			(SELECT COUNT(*) FROM tags) as tags,
			(SELECT COUNT(*) FROM articles WHERE synced_at IS NOT NULL) as synced_articles,
			(SELECT COUNT(*) FROM articles WHERE failed_count >= 5) as failed_articles
	`

	if err := database.Get(&counts, countQuery); err != nil {
		return fmt.Errorf("failed to get counts: %w", err)
	}

	fmt.Printf("Database Statistics:\n")
	fmt.Printf("  Articles: %d\n", counts.Articles)
	fmt.Printf("  Folders: %d\n", counts.Folders)
	fmt.Printf("  Tags: %d\n", counts.Tags)
	fmt.Printf("  Synced Articles: %d\n", counts.SyncedArticles)
	fmt.Printf("  Failed Articles (5+ failures): %d\n", counts.FailedArticles)

	fmt.Println("\nUpdating folder paths...")
	if err := database.UpdateFolderPaths(); err != nil {
		return fmt.Errorf("failed to update folder paths: %w", err)
	}

	fmt.Println("\nRebuilding FTS index...")
	if _, err := database.Exec("INSERT INTO articles_fts(articles_fts) VALUES('rebuild')"); err != nil {
		fmt.Printf("Warning: FTS rebuild failed: %v\n", err)
	}

	var duplicateURLs []struct {
		URL   string `db:"url"`
		Count int    `db:"count"`
	}

	duplicateQuery := `
		SELECT url, COUNT(*) as count
		FROM articles
		GROUP BY url
		HAVING COUNT(*) > 1
	`

	if err := database.Select(&duplicateURLs, duplicateQuery); err == nil && len(duplicateURLs) > 0 {
		fmt.Printf("\nWarning: Found %d duplicate URLs:\n", len(duplicateURLs))
		for _, dup := range duplicateURLs {
			fmt.Printf("  %s (%d copies)\n", dup.URL, dup.Count)
		}
	}

	fmt.Println("\nDatabase doctor completed successfully!")
	return nil
}

func runMCP(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "Starting MCP server for instapaper-cli v1.2.1\n")
	fmt.Fprintf(os.Stderr, "Database: %s\n", dbPath)
	fmt.Fprintf(os.Stderr, "MCP server listening on stdio...\n")

	// Create and start MCP server
	server := mcp.NewServer(database)
	return server.Start()
}

func runObsolete(cmd *cobra.Command, args []string) error {
	ids, _ := cmd.Flags().GetInt64Slice("ids")
	statusCodes, _ := cmd.Flags().GetIntSlice("status-codes")
	minFailures, _ := cmd.Flags().GetInt("min-failures")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	confirm, _ := cmd.Flags().GetBool("confirm")

	// Validate that at least one criteria is provided
	if len(ids) == 0 && len(statusCodes) == 0 && minFailures == 0 {
		return fmt.Errorf("must specify at least one criteria: --ids, --status-codes, or --min-failures")
	}

	// Require confirmation for non-dry-run operations
	if !dryRun && !confirm {
		return fmt.Errorf("must use --confirm flag for non-dry-run operations")
	}

	// Build query conditions
	var conditions []string
	var queryArgs []interface{}

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			queryArgs = append(queryArgs, id)
		}
		conditions = append(conditions, fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(statusCodes) > 0 {
		placeholders := make([]string, len(statusCodes))
		for i, code := range statusCodes {
			placeholders[i] = "?"
			queryArgs = append(queryArgs, code)
		}
		conditions = append(conditions, fmt.Sprintf("status_code IN (%s)", strings.Join(placeholders, ",")))
	}

	if minFailures > 0 {
		conditions = append(conditions, "failed_count >= ?")
		queryArgs = append(queryArgs, minFailures)
	}

	// Add condition to exclude already obsolete articles
	conditions = append(conditions, "obsolete = FALSE")

	whereClause := strings.Join(conditions, " AND ")

	// First, get the articles that would be affected
	selectQuery := fmt.Sprintf(`
		SELECT id, url, title, status_code, failed_count
		FROM articles
		WHERE %s
		ORDER BY id
	`, whereClause)

	type ObsoleteCandidate struct {
		ID          int64  `db:"id"`
		URL         string `db:"url"`
		Title       string `db:"title"`
		StatusCode  *int   `db:"status_code"`
		FailedCount int    `db:"failed_count"`
	}

	var candidates []ObsoleteCandidate
	if err := database.Select(&candidates, selectQuery, queryArgs...); err != nil {
		return fmt.Errorf("failed to find articles: %w", err)
	}

	if len(candidates) == 0 {
		fmt.Println("No articles found matching the criteria.")
		return nil
	}

	// Show articles that would be obsoleted
	for _, article := range candidates {
		statusStr := "unknown"
		if article.StatusCode != nil {
			statusStr = fmt.Sprintf("%d", *article.StatusCode)
		}
		fmt.Printf("  ID: %d | Status: %s | Failures: %d\n", article.ID, statusStr, article.FailedCount)
		fmt.Printf("  URL: %s\n", article.URL)
		fmt.Printf("  Title: %s\n\n", article.Title)
	}

	fmt.Printf("Found %d articles to mark as obsolete.\n", len(candidates))

	if dryRun {
		fmt.Println("Dry run completed. Use --confirm to actually mark these articles as obsolete.")
		return nil
	}

	// Execute the update
	updateQuery := fmt.Sprintf(`
		UPDATE articles
		SET obsolete = TRUE
		WHERE %s
	`, whereClause)

	result, err := database.Exec(updateQuery, queryArgs...)
	if err != nil {
		return fmt.Errorf("failed to mark articles as obsolete: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("Successfully marked %d articles as obsolete.\n", rowsAffected)
	return nil
}

func runListObsolete(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	limit, _ := cmd.Flags().GetInt("limit")

	query := `
		SELECT id, url, title, folder_id, instapapered_at, status_code, failed_count
		FROM articles
		WHERE obsolete = TRUE
		ORDER BY instapapered_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	type ObsoleteArticle struct {
		ID             int64  `db:"id" json:"id"`
		URL            string `db:"url" json:"url"`
		Title          string `db:"title" json:"title"`
		FolderID       *int64 `db:"folder_id" json:"folder_id,omitempty"`
		InstapaperedAt string `db:"instapapered_at" json:"instapapered_at"`
		StatusCode     *int   `db:"status_code" json:"status_code,omitempty"`
		FailedCount    int    `db:"failed_count" json:"failed_count"`
	}

	var articles []ObsoleteArticle
	if err := database.Select(&articles, query); err != nil {
		return fmt.Errorf("failed to query obsolete articles: %w", err)
	}

	if len(articles) == 0 {
		fmt.Println("No obsolete articles found.")
		return nil
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(articles)
	}

	fmt.Printf("Found %d obsolete articles:\n\n", len(articles))
	for _, article := range articles {
		statusStr := "unknown"
		if article.StatusCode != nil {
			statusStr = fmt.Sprintf("%d", *article.StatusCode)
		}

		fmt.Printf("ID: %d | Status: %s | Failures: %d\n", article.ID, statusStr, article.FailedCount)
		fmt.Printf("Added: %s\n", article.InstapaperedAt)
		fmt.Printf("URL: %s\n", article.URL)
		fmt.Printf("Title: %s\n\n", article.Title)
	}

	return nil
}

func getStatusCodeName(code string) string {
	switch code {
	case "200":
		return "OK"
	case "201":
		return "Created"
	case "202":
		return "Accepted"
	case "301":
		return "Moved Permanently"
	case "302":
		return "Found"
	case "304":
		return "Not Modified"
	case "400":
		return "Bad Request"
	case "401":
		return "Unauthorized"
	case "403":
		return "Forbidden"
	case "404":
		return "Not Found"
	case "429":
		return "Too Many Requests"
	case "500":
		return "Internal Server Error"
	case "502":
		return "Bad Gateway"
	case "503":
		return "Service Unavailable"
	case "504":
		return "Gateway Timeout"
	default:
		return "Unknown"
	}
}

func runStats(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Define the stats structure
	type DatabaseStats struct {
		Total       int                    `json:"total"`
		Obsolete    int                    `json:"obsolete"`
		Fetched     int                    `json:"fetched"`
		NotFetched  int                    `json:"not_fetched"`
		Failures    map[string]int         `json:"failures_by_count"`
		StatusCodes map[string]int         `json:"status_codes"`
		Summary     map[string]interface{} `json:"summary,omitempty"`
	}

	var stats DatabaseStats
	stats.Failures = make(map[string]int)
	stats.StatusCodes = make(map[string]int)

	// Get total articles
	if err := database.Get(&stats.Total, "SELECT COUNT(*) FROM articles"); err != nil {
		return fmt.Errorf("failed to get total count: %w", err)
	}

	// Get obsolete articles
	if err := database.Get(&stats.Obsolete, "SELECT COUNT(*) FROM articles WHERE obsolete = TRUE"); err != nil {
		return fmt.Errorf("failed to get obsolete count: %w", err)
	}

	// Get fetched articles (have content)
	if err := database.Get(&stats.Fetched, "SELECT COUNT(*) FROM articles WHERE synced_at IS NOT NULL AND obsolete = FALSE"); err != nil {
		return fmt.Errorf("failed to get fetched count: %w", err)
	}

	// Get not fetched articles
	if err := database.Get(&stats.NotFetched, "SELECT COUNT(*) FROM articles WHERE synced_at IS NULL AND obsolete = FALSE"); err != nil {
		return fmt.Errorf("failed to get not fetched count: %w", err)
	}

	// Get failure statistics by count (non-obsolete only)
	failureQuery := `
		SELECT failed_count, COUNT(*) as count
		FROM articles
		WHERE failed_count > 0 AND obsolete = FALSE
		GROUP BY failed_count
		ORDER BY failed_count
	`

	type FailureCount struct {
		FailedCount int `db:"failed_count"`
		Count       int `db:"count"`
	}

	var failures []FailureCount
	if err := database.Select(&failures, failureQuery); err != nil {
		return fmt.Errorf("failed to get failure statistics: %w", err)
	}

	// Convert to map for easier access
	for _, f := range failures {
		stats.Failures[fmt.Sprintf("%d", f.FailedCount)] = f.Count
	}

	// Get status code statistics (failed, non-obsolete only)
	statusQuery := `
		SELECT status_code, COUNT(*) as count
		FROM articles
		WHERE status_code IS NOT NULL AND status_code != 0 AND status_code != 200 AND obsolete = FALSE
		GROUP BY status_code
		ORDER BY status_code
	`

	type StatusCode struct {
		StatusCode int `db:"status_code"`
		Count      int `db:"count"`
	}

	var statusCodes []StatusCode
	if err := database.Select(&statusCodes, statusQuery); err != nil {
		return fmt.Errorf("failed to get status code statistics: %w", err)
	}

	// Convert to map for easier access
	for _, s := range statusCodes {
		stats.StatusCodes[fmt.Sprintf("%d", s.StatusCode)] = s.Count
	}

	// Calculate summary percentages for human-readable output
	if !jsonOutput {
		stats.Summary = map[string]interface{}{
			"active_articles":    stats.Total - stats.Obsolete,
			"fetch_success_rate": float64(stats.Fetched) / float64(stats.Total-stats.Obsolete) * 100,
			"obsolete_rate":      float64(stats.Obsolete) / float64(stats.Total) * 100,
		}
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)
	}

	// Human-readable output
	fmt.Printf("Database Statistics\n")
	fmt.Printf("==================\n\n")

	fmt.Printf("Articles Overview:\n")
	fmt.Printf("  Total Articles:     %d\n", stats.Total)
	fmt.Printf("  Active Articles:    %d (%.1f%%)\n", stats.Total-stats.Obsolete,
		float64(stats.Total-stats.Obsolete)/float64(stats.Total)*100)
	fmt.Printf("  Obsolete Articles:  %d (%.1f%%)\n", stats.Obsolete,
		float64(stats.Obsolete)/float64(stats.Total)*100)

	fmt.Printf("\nFetch Status (Active Articles):\n")
	fmt.Printf("  Successfully Fetched: %d (%.1f%%)\n", stats.Fetched,
		float64(stats.Fetched)/float64(stats.Total-stats.Obsolete)*100)
	fmt.Printf("  Not Yet Fetched:     %d (%.1f%%)\n", stats.NotFetched,
		float64(stats.NotFetched)/float64(stats.Total-stats.Obsolete)*100)

	if len(stats.Failures) > 0 {
		fmt.Printf("\nFetch Failures (Active Articles):\n")
		totalFailed := 0
		for failCount, count := range stats.Failures {
			fmt.Printf("  %s failure(s): %d articles\n", failCount, count)
			totalFailed += count
		}
		fmt.Printf("  Total with failures: %d (%.1f%% of active)\n", totalFailed,
			float64(totalFailed)/float64(stats.Total-stats.Obsolete)*100)
	} else {
		fmt.Printf("\nFetch Failures: None\n")
	}

	if len(stats.StatusCodes) > 0 {
		fmt.Printf("\nFailed HTTP Status Codes (Active Articles):\n")

		// Sort status codes numerically
		var sortedCodes []string
		for code := range stats.StatusCodes {
			sortedCodes = append(sortedCodes, code)
		}
		sort.Slice(sortedCodes, func(i, j int) bool {
			// Convert to int for numeric comparison
			codeI, _ := strconv.Atoi(sortedCodes[i])
			codeJ, _ := strconv.Atoi(sortedCodes[j])
			return codeI < codeJ
		})

		for _, statusCode := range sortedCodes {
			count := stats.StatusCodes[statusCode]
			statusName := getStatusCodeName(statusCode)
			fmt.Printf("  %s (%s): %d articles\n", statusCode, statusName, count)
		}
	}

	// Health recommendations
	fmt.Printf("\nHealth Summary:\n")
	if stats.Obsolete > 0 {
		fmt.Printf("  📁 %d obsolete articles excluded from operations\n", stats.Obsolete)
	}
	if stats.NotFetched > 0 {
		fmt.Printf("  ⏳ %d articles ready for content fetching\n", stats.NotFetched)
	}

	// Check for high failure articles that might need obsoleting
	for failCount, count := range stats.Failures {
		if failCount >= "4" {
			fmt.Printf("  ⚠️  %d articles with %s+ failures (consider marking obsolete)\n", count, failCount)
		}
	}

	if len(stats.Failures) == 0 && stats.NotFetched == 0 {
		fmt.Printf("  ✅ All active articles successfully fetched!\n")
	}

	return nil
}