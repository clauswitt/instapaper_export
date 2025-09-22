# Instapaper CLI

A full-featured Go CLI application that transforms Instapaper CSV exports into a searchable, fetchable, and exportable knowledge base.

## Features

- **Import**: Parse Instapaper CSV exports into SQLite database with proper normalization
- **Fetch**: Download article content using readability extraction
- **Search**: Full-text and LIKE search across titles, URLs, content, folders, and tags
- **Export**: Generate Markdown files with YAML frontmatter for knowledge management
- **MCP Server**: Model Context Protocol server for AI integration (Claude, etc.)
- **Manage**: Utilities for folders, tags, and database maintenance

## Installation

### Download Binary

Download the latest binary from the [Releases](https://github.com/clauswitt/instapaper_export/releases) page.

### Build from Source

```bash
git clone https://github.com/clauswitt/instapaper_export.git
cd instapaper_export
go build -o instapaper-cli cmd/instapaper-cli/main.go
```

## Quick Start

```bash
# 1. Import your Instapaper CSV export
instapaper-cli import --csv export.csv

# 2. Fetch article content (optional but recommended)
instapaper-cli fetch --limit 50

# 3. Search your articles
instapaper-cli search "kubernetes"

# 4. Export to markdown files
instapaper-cli export-all --dir ~/knowledge-base

# 5. Start MCP server for AI integration
instapaper-cli mcp
```

## Commands

### Import
Import articles from Instapaper CSV export:
```bash
instapaper-cli import --csv path/to/export.csv
```

### Fetch
Download article content with readability extraction:
```bash
# Fetch all unfetched articles
instapaper-cli fetch

# Fetch with limit and rate limiting
instapaper-cli fetch --limit 100 --delay 1s

# Retry failed fetches
instapaper-cli fetch --retry-failed
```

### Search
Search through your articles:
```bash
# Full-text search
instapaper-cli search "machine learning"

# Search specific fields
instapaper-cli search --title "docker"
instapaper-cli search --folder "tech"
instapaper-cli search --tag "ai"

# Output as JSON
instapaper-cli search "golang" --json
```

### Export
Export individual articles or entire collection:
```bash
# Export single article
instapaper-cli export --id 123 --output article.md

# Export all articles to directory
instapaper-cli export-all --dir ~/knowledge-base

# Preserve folder structure
instapaper-cli export-all --dir ~/kb --preserve-folders
```

### MCP Server
Start Model Context Protocol server for AI integration:
```bash
# Start MCP server (listens on stdio)
instapaper-cli mcp

# Start with specific database
instapaper-cli mcp --db /path/to/instapaper.sqlite
```

**Available MCP Tools:**
- `search_articles` - Search with filters, full-text search, date ranges
- `get_article` - Get single article with full content by ID
- `list_folders` - Browse available folders with article counts
- `list_tags` - Browse available tags with article counts
- `export_articles` - Export filtered articles to markdown for AI consumption

**Claude Desktop Integration:**
```json
{
  "mcpServers": {
    "instapaper": {
      "command": "/path/to/instapaper-cli",
      "args": ["mcp", "--db", "/path/to/instapaper.sqlite"]
    }
  }
}
```

### Management
Manage folders, tags, and database:
```bash
# List folders
instapaper-cli folders

# List tags
instapaper-cli tags

# Database health check
instapaper-cli doctor

# Show version
instapaper-cli version
```

## Architecture

- **SQLite backend** with migration system and FTS5 full-text search
- **Cobra CLI** framework with subcommands
- **Readability extraction** for clean article content using go-shiori/go-readability
- **HTML-to-Markdown** conversion with cleanup
- **URL canonicalization** and duplicate handling
- **Retry logic** with exponential backoff for failed fetches

## Tech Stack

- Go 1.21+
- SQLite (modernc.org/sqlite - pure Go implementation)
- Cobra CLI framework
- go-shiori/go-readability for content extraction
- html-to-markdown for clean conversion
- gosimple/slug for filename generation

## Use Cases

Perfect for turning your Instapaper exports into:
- Personal knowledge base
- Research archive
- Searchable article collection
- Markdown-based wiki
- Content for static site generators

## Configuration

The CLI uses these defaults:
- Database: `instapaper.sqlite` in current directory
- Migrations: `migrations/` directory
- Export format: Markdown with YAML frontmatter

## License

MIT License - see LICENSE file for details.