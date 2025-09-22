package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"instapaper-cli/internal/db"
	"instapaper-cli/internal/export"
	"instapaper-cli/internal/search"
)

// Server represents the MCP server for Instapaper
type Server struct {
	db       *db.DB
	search   *search.Search
	export   *export.Export
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server instance
func NewServer(database *db.DB) *Server {
	s := &Server{
		db:     database,
		search: search.New(database),
		export: export.New(database),
	}

	// Create MCP server
	s.mcpServer = server.NewMCPServer(
		"instapaper",
		"1.0.0",
	)

	s.registerTools()
	return s
}

// Start starts the MCP server using stdio
func (s *Server) Start() error {
	return server.ServeStdio(s.mcpServer)
}

// registerTools registers all available MCP tools
func (s *Server) registerTools() {
	// Search articles tool
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "search_articles",
		Description: "Search articles with various filters including full-text search (default), date ranges, tags, and folders. Multiple keywords in query are treated as intersection (AND).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query text. Multiple keywords will be treated as AND (intersection). Use full-text search for better results.",
				},
				"field": map[string]interface{}{
					"type":        "string",
					"description": "Specific field to search: url, title, content, tags, folder",
					"enum":        []string{"url", "title", "content", "tags", "folder"},
				},
				"use_fts": map[string]interface{}{
					"type":        "boolean",
					"description": "Use full-text search (default: true). FTS is faster, more accurate, and supports intersection queries. Set to false to use LIKE search instead.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 50)",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Filter by specific tags",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"folders": map[string]interface{}{
					"type":        "array",
					"description": "Filter by specific folders",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"date_after": map[string]interface{}{
					"type":        "string",
					"description": "Only include articles added after this date (ISO 8601 format)",
				},
				"date_before": map[string]interface{}{
					"type":        "string",
					"description": "Only include articles added before this date (ISO 8601 format)",
				},
				"only_synced": map[string]interface{}{
					"type":        "boolean",
					"description": "Only return articles that have content downloaded",
				},
			},
		},
	}, s.handleSearchArticles)

	// Get single article tool
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "get_article",
		Description: "Get a single article by ID with full content and metadata",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "integer",
					"description": "Article ID",
				},
				"include_content": map[string]interface{}{
					"type":        "boolean",
					"description": "Include full markdown content (default: true)",
				},
				"include_html": map[string]interface{}{
					"type":        "boolean",
					"description": "Include raw HTML content",
				},
				"include_tags": map[string]interface{}{
					"type":        "boolean",
					"description": "Include tags array (default: true)",
				},
			},
			Required: []string{"id"},
		},
	}, s.handleGetArticle)

	// List folders tool
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "list_folders",
		Description: "Get all available folders with article counts",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, s.handleListFolders)

	// List tags tool
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "list_tags",
		Description: "Get all available tags with article counts",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"min_count": map[string]interface{}{
					"type":        "integer",
					"description": "Only include tags with at least this many articles",
				},
			},
		},
	}, s.handleListTags)

	// Export articles tool
	s.mcpServer.AddTool(mcp.Tool{
		Name:        "export_articles",
		Description: "Export articles to markdown format with filtering options. Returns content directly for AI consumption.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query to filter articles",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Filter by specific tags",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of articles to export",
				},
				"only_synced": map[string]interface{}{
					"type":        "boolean",
					"description": "Only export articles with downloaded content (default: true)",
				},
			},
		},
	}, s.handleExportArticles)
}