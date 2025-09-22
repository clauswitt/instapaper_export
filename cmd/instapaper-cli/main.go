package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/export"
	"instapaper-cli/internal/fetcher"
	"instapaper-cli/internal/importer"
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
	)

	searchCmd.Flags().StringVar(&searchField, "field", "", "Search specific field: url, title, content, tags, folder")
	searchCmd.Flags().BoolVar(&searchFTS, "fts", false, "Use full-text search")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "Maximum number of results")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Output results as JSON")

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
			cmd.Println("instapaper-cli v1.0.0")
		},
	}

	rootCmd.AddCommand(importCmd, fetchCmd, searchCmd, exportCmd, exportAllCmd, foldersCmd, tagsCmd, doctorCmd, versionCmd)

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

	opts := search.SearchOptions{
		Query:      query,
		Field:      field,
		UseFTS:     useFTS,
		Limit:      limit,
		JSONOutput: jsonOutput,
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