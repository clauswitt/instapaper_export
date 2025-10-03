# Instapaper CLI

A full-featured Go CLI application that transforms Instapaper CSV exports into a searchable, fetchable, and exportable knowledge base.

## Features

- **Import**: Parse Instapaper CSV exports into SQLite database with proper normalization
- **RSS Feeds**: Subscribe to and sync Instapaper RSS feeds with tag inheritance
- **Fetch**: Download article content using readability extraction with smart retry logic
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

# 2. Or subscribe to your Instapaper RSS feed
instapaper-cli rss:add https://www.instapaper.com/rss/YOUR_FEED_ID --tags "instapaper,reading"
instapaper-cli rss

# 3. Fetch article content (optional but recommended)
instapaper-cli fetch --limit 50

# 4. Search your articles
instapaper-cli search "kubernetes"

# 5. Export to markdown files
instapaper-cli export-all --dir ~/knowledge-base

# 6. Start MCP server for AI integration
instapaper-cli mcp
```

## Commands

### Import
Import articles from Instapaper CSV export:
```bash
instapaper-cli import --csv path/to/export.csv
```

### RSS Feeds
Manage and sync Instapaper RSS feeds:
```bash
# Add a new RSS feed (with optional tags)
instapaper-cli rss:add https://www.instapaper.com/rss/YOUR_FEED_ID
instapaper-cli rss:add https://www.instapaper.com/rss/YOUR_FEED_ID --name "My Reading List" --tags "tech,articles"

# List all RSS feeds
instapaper-cli rss:list
instapaper-cli rss:list --json

# Sync all active RSS feeds
instapaper-cli rss

# Update a feed's name or tags
instapaper-cli rss:update --id 1 --name "New Name"
instapaper-cli rss:update --id 1 --tags "new,tags"

# Delete a feed (articles remain in database)
instapaper-cli rss:delete --id 1
```

**Features:**
- Automatic duplicate prevention (URL-based)
- URL normalization (http→https) to avoid duplicates
- Tag inheritance: all articles from a feed get the feed's tags
- Feed-level tag management without affecting existing articles

### Fetch
Download article content with readability extraction:
```bash
# Fetch all unfetched articles
instapaper-cli fetch

# Fetch with limit
instapaper-cli fetch --limit 100

# Fetch oldest articles first (default)
instapaper-cli fetch --order oldest --limit 50

# Fetch newest articles first
instapaper-cli fetch --order newest --limit 50
```

**Smart Retry Logic:**
- Articles that fail are automatically retried after 1 hour
- Maximum 5 retry attempts before permanent exclusion
- Failed articles can be marked as obsolete to exclude them completely

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

# Export search results directly
instapaper-cli export-all --dir ~/exports --from-search "kubernetes"
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

# Show database statistics
instapaper-cli stats

# Show version
instapaper-cli version

# Mark articles as obsolete (exclude from searches/exports)
instapaper-cli obsolete --status-codes 404,403 --confirm
instapaper-cli obsolete --min-failures 3 --confirm
instapaper-cli obsolete --ids 123,456 --confirm

# List obsolete articles
instapaper-cli list-obsolete

# Preview what would be marked obsolete (dry run)
instapaper-cli obsolete --status-codes 404 --dry-run
```

## Architecture

- **SQLite backend** with migration system and FTS5 full-text search
- **Foreign key constraints** properly enabled for referential integrity
- **Cobra CLI** framework with subcommands
- **RSS feed management** with CRUD operations and tag inheritance
- **Readability extraction** for clean article content using go-shiori/go-readability
- **HTML-to-Markdown** conversion with cleanup
- **URL normalization** (http→https) and duplicate prevention
- **Smart retry logic** with 1-hour cooldown and 5-attempt maximum

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