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

# Search with date filtering
instapaper-cli search "kubernetes" --since "1w"
instapaper-cli search "ai" --since "today"
instapaper-cli search "golang" --since "2024-01-01" --until "2024-06-01"

# Output as JSON
instapaper-cli search "golang" --json
```

### Latest Articles
Get the most recent articles with optional date filtering:
```bash
# Get latest 20 articles
instapaper-cli latest

# Get articles from last week
instapaper-cli latest --since "1w"

# Get articles from today
instapaper-cli latest --since "today" --limit 10

# Get articles from specific date range
instapaper-cli latest --since "2024-01-01" --until "2024-01-31"

# Output as JSON
instapaper-cli latest --json
```

**Date Filter Examples:**
- `today` - Articles from today
- `yesterday` - Articles from yesterday
- `1d` - Articles from last 1 day
- `1w` - Articles from last 1 week
- `1m` - Articles from last 1 month
- `2024-01-15` - Articles from specific date
- `2024-01-15T10:00:00Z` - Articles from specific datetime

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
- `search_articles` - Search with filters, full-text search, date ranges (supports "kubernetes" + since="1w")
- `get_article` - Get single article with full content by ID
- `get_latest_articles` - Get recent articles with date filtering (1d, 1w, today, etc.)
- `list_folders` - Browse available folders with article counts
- `list_tags` - Browse available tags with article counts
- `export_articles` - Export filtered articles to markdown for AI consumption
- `get_usage_examples` - Get examples of how to handle common user requests

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