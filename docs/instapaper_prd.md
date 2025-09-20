# PRD — `instapaper-cli` (Go)

A single-binary Go **CLI** to ingest an Instapaper CSV, manage a local SQLite library, fetch readable content (Markdown), search, and export files.

---

## 1) Goals & Non-Goals

### Goals
- Import Instapaper CSV into a local **SQLite** DB (`instapaper.sqlite`).
- Normalize **folders** (nestable) and **tags** (many-to-many).
- Fetch article content using **readability** and store pretty Markdown.
- Reliable incremental syncing with backoff & retry rules.
- Powerful CLI search + export (single & bulk) with YAML frontmatter.
- Deterministic filenames; safe handling of duplicates/renames.

### Non-Goals
- Web UI, scheduling daemon, or cloud sync.
- Auth to Instapaper API (CSV is the source of truth initially).
- Rendering PDFs/EPUBs (future extension).

---

## 2) Data Model (SQLite)

**DB file:** `instapaper.sqlite` (created if missing).  
All timestamps stored as **UTC ISO8601** (`TEXT`, e.g., `2025-09-20T12:34:56Z`). Integers only where noted.

### Tables

#### `articles`
| column | type | notes |
|---|---|---|
| `id` | INTEGER PK | autoincrement |
| `url` | TEXT UNIQUE | canonicalized (strip fragments, trim) |
| `title` | TEXT | from CSV |
| `selection` | TEXT NULL | CSV “Selection” (often empty) |
| `folder_id` | INTEGER NULL | FK → `folders.id` |
| `instapapered_at` | TEXT | converted from CSV `Timestamp` (Unix seconds → UTC) |
| `synced_at` | TEXT NULL | last successful content fetch time |
| `sync_failed_at` | TEXT NULL | last failure time |
| `failed_count` | INTEGER DEFAULT 0 | consecutive failures |
| `status_code` | INTEGER NULL | last HTTP status |
| `status_text` | TEXT NULL | last HTTP status text / error summary |
| `final_url` | TEXT NULL | after redirects |
| `content_md` | TEXT NULL | **Markdown** result |
| `raw_html` | TEXT NULL | optional: readability raw HTML (for debug) |

**Indexes**
- `idx_articles_folder` on `folder_id`
- `idx_articles_instapapered_at`
- `idx_articles_synced_at`
- `idx_articles_failed` on (`failed_count`, `sync_failed_at`)
- `idx_articles_title`
- `idx_articles_url` UNIQUE

#### `folders`
| column | type | notes |
|---|---|---|
| `id` | INTEGER PK |  |
| `title` | TEXT UNIQUE | Use **title** as the unique human label (per requirement) |
| `parent_id` | INTEGER NULL | FK → `folders.id` (nestable) |
| `path_cache` | TEXT | computed path like `Parent/Child/Sub` for quick export |

> **Note:** The CSV only contains a single folder label (e.g., `Unread`/`Archive`). Nesting is supported in the schema; initial import will create a single-level folder. Users may later edit folder hierarchy via CLI.

#### `tags`
| column | type | notes |
|---|---|---|
| `id` | INTEGER PK | |
| `title` | TEXT UNIQUE | Use **title** instead of “name” |

#### `article_tags` (M-M)
| column | type |
|---|---|
| `article_id` | INTEGER FK → `articles.id` |
| `tag_id` | INTEGER FK → `tags.id` |
**Composite PK:** `(article_id, tag_id)`

#### Full-Text Search (optional but recommended)
Use **FTS5** over `url`, `title`, `content_md`, plus denormalized folder/tag strings:

- Virtual table `articles_fts (url, title, content, folder, tags, content='')`
- Triggers keep it in sync with `articles`/relations:
  - On insert/update/delete of `articles` and `article_tags`.

---

## 3) Importer (CSV → DB)

**Input columns** (observed):  
`URL,Title,Selection,Folder,Timestamp,Tags`

Rules:
- `Timestamp` is **Unix seconds** → convert to UTC ISO8601 as `instapapered_at`.
- `Folder`: ensure folder exists; create if missing (`title` as unique key).
- `Tags`: CSV field may be `[]` or a JSON list; support both JSON (`["a","b"]`) and comma-separated strings. Trim and dedupe; create tags by `title`.
- Upsert articles by **URL** (unique). If an article exists, update `title`, `selection`, `folder_id`, `instapapered_at`, and tags.
- Leave `synced_at`, `content_md`, etc., untouched during import.

**CLI**
```
instapaper-cli import --csv path/to/export.csv
```

**Edge cases**
- Empty/malformed rows → skip with warning, continue.
- Future timestamps → still convert; no special treatment.
- Duplicate URLs in CSV → keep first, warn on subsequent.

---

## 4) Content Fetcher

Separate command (no import side effects).

### Behavior
- **Selection** of candidates:
  - Exclude rows where `synced_at IS NOT NULL`.
  - Exclude rows where `failed_count >= 5` **unless** `sync_failed_at <= now-1h`.
  - Optional `search_phrase` filter across url/title/content/tags/folder.
  - Order:
    - `--order oldest` → by `instapapered_at ASC`
    - `--order newest` → by `instapapered_at DESC`
  - `--limit N` (default **10**, max e.g. 200 to be safe).

- **Fetching**
  - HTTP client with:
    - Redirects **enabled** (store final URL in `final_url`).
    - Reasonable timeout (e.g., 20s) & user agent.
    - Gzip/deflate support.
  - On 200 responses:
    - Run **readability** (Go: `github.com/go-shiori/go-readability`) to extract main content and title (don’t overwrite CSV title unless `--prefer-extracted-title` is provided).
    - Convert resulting HTML → **Markdown** (e.g., `github.com/JohannesKaufmann/html-to-markdown` or goldmark via HTML → MD).
    - Run a **prettifier** step: normalize headings, lists, code blocks, blockquotes, remove tracker junk.
    - Save to `content_md` (and `raw_html` if `--store-raw` set).
    - Set `synced_at = now()`, `status_code=200`, `status_text="OK"`, reset `failed_count=0`, `sync_failed_at=NULL`.

  - On non-200 or parse failure:
    - Set `sync_failed_at = now()`
    - Increment `failed_count`
    - Set `status_code` to HTTP status (or `0` if network/parse error)
    - Set `status_text` to short reason
    - **Log** message (stderr & optional file with `--log path.log`)

- **No immediate retry** in the same run. Next runs may pick up if `failed_count < 5` and last failure ≥ 1h ago.

**CLI**
```
instapaper-cli fetch   [--order oldest|newest]   [--search "phrase"]   [--limit 10]   [--prefer-extracted-title]   [--store-raw]   [--log fetch.log]
```

---

## 5) Search

Two modes: quick (LIKE) and FTS.

**CLI**
```
# general search across url, title, content, tags, folder
instapaper-cli search "deep learning"

# field-specific
instapaper-cli search --field url "nytimes.com"
instapaper-cli search --field title "postgres"
instapaper-cli search --field content "kubernetes"
instapaper-cli search --field tags "ai"
instapaper-cli search --field folder "Unread"

# options
  [--limit 50] [--fts] [--json]
```

Output: table to stdout by default; `--json` for scripts (id, url, title, folder path, tags, synced_at, failed_count, status_code).

---

## 6) Export (single & bulk)

### Frontmatter (YAML)
Every exported Markdown file begins with:

```yaml
---
title: "<article title>"
instapapered_at: "<ISO8601>"
exported_at: "<now UTC ISO8601>"
source: "<url>"
tags: ["instapaper", "<tag1>", "<tag2>", ...]
---
```

### File naming
- Default filename: slugified **title** (ascii, lowercase, `-` separators, max 120 chars).
- If collision, append `-<id>` (or `-2`, `-3`…).
- Extension: `.md`.

### Paths
- **export-one**: explicit path or stdout.
- **export-all**: base directory + nested subfolders using `folders.path_cache`. If folder is NULL → store in base dir.

**CLI**
```
# single
instapaper-cli export --id 123 --out /path/to/file.md
instapaper-cli export --id 123 --stdout

# bulk
instapaper-cli export-all --dir ~/Archives/Instapaper
# options
  [--only-synced] [--include-unsynced] [--folder "Parent/Child"] [--tag ai] [--since 2025-01-01] [--until 2025-12-31]
```

Rules:
- Default `export-all` exports **only articles with `content_md IS NOT NULL`**. Use `--include-unsynced` to write stub files (frontmatter + link, empty body).
- Create directories as needed. Preserve/escape safe characters.

---

## 7) CLI Summary

```
instapaper-cli
  import        --csv <path>
  fetch         [--order oldest|newest] [--search "phrase"] [--limit 10]
                [--prefer-extracted-title] [--store-raw] [--log <path>]
  search        ["query"] [--field url|title|content|tags|folder] [--fts] [--limit N] [--json]
  export        --id <article_id> [--out <path>|--stdout]
  export-all    --dir <path> [--only-synced|--include-unsynced]
                [--folder "A/B"] [--tag t] [--since <ISO>] [--until <ISO>]
  folders       [list|mv|mkdir]   # manage nesting, update path_cache
  tags          [list|rename]
  doctor        # db integrity, fts rebuild, url canonicalization
  version
```

---

## 8) Implementation Notes (Go)

### Dependencies
- CSV: `encoding/csv`
- SQLite: `modernc.org/sqlite` (pure Go) **or** `github.com/mattn/go-sqlite3` (CGO). Prefer pure Go for easy builds.
- Migrations: `github.com/jmoiron/sqlx` + simple migration files, or `pressly/goose`.
- Readability: `github.com/go-shiori/go-readability`
- HTML→Markdown: `github.com/JohannesKaufmann/html-to-markdown`
- Slugify: `github.com/gosimple/slug`
- CLI: `github.com/spf13/cobra` + `viper` for flags/config
- Logging: `log/slog` or `zerolog`

### Packages
```
/cmd/instapaper-cli
/internal/db          // schema, queries, migrations
/internal/importer    // csv -> db
/internal/fetcher     // http, readability, md conversion
/internal/search      // fts/like queries
/internal/export      // file naming, frontmatter, writers
/internal/model       // Go structs
/internal/util        // time, slug, path, errors
/migrations           // .sql files
```

### Migrations (sketch)

**0001_init.sql**
```sql
PRAGMA foreign_keys=ON;

CREATE TABLE folders (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL UNIQUE,
  parent_id INTEGER REFERENCES folders(id) ON DELETE SET NULL,
  path_cache TEXT
);

CREATE TABLE articles (
  id INTEGER PRIMARY KEY,
  url TEXT NOT NULL UNIQUE,
  title TEXT,
  selection TEXT,
  folder_id INTEGER REFERENCES folders(id) ON DELETE SET NULL,
  instapapered_at TEXT NOT NULL,
  synced_at TEXT,
  sync_failed_at TEXT,
  failed_count INTEGER NOT NULL DEFAULT 0,
  status_code INTEGER,
  status_text TEXT,
  final_url TEXT,
  content_md TEXT,
  raw_html TEXT
);

CREATE TABLE tags (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL UNIQUE
);

CREATE TABLE article_tags (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (article_id, tag_id)
);

CREATE INDEX idx_articles_folder ON articles(folder_id);
CREATE INDEX idx_articles_instapapered_at ON articles(instapapered_at);
CREATE INDEX idx_articles_synced_at ON articles(synced_at);
CREATE INDEX idx_articles_failed ON articles(failed_count, sync_failed_at);
```

**0002_fts.sql** (optional)
```sql
CREATE VIRTUAL TABLE articles_fts USING fts5(
  url, title, content, folder, tags, content=''
);

-- Simple triggers to keep FTS updated (implementation detail in code or SQL).
```

### URL Canonicalization
- Normalize scheme to https where possible, strip trailing slashes, remove fragments, keep query string.

### Backoff & Retry
- Selection excludes `failed_count >= 5` **unless** `sync_failed_at <= now-1h`.
- No per-run retry.

### HTTP
- Follow redirects (default Go client does).
- Record `status_code` and a terse `status_text` (“OK”, “Timeout”, “ReadabilityError”, etc.).
- Save `final_url` after following redirects.

### Markdown “prettify”
- Convert `<h1>`→`#`, preserve `<code>`, `<pre>`, `<blockquote>`.
- Remove empty paragraphs, decode entities, collapse whitespace.

---

## 9) Acceptance Criteria

- **Import**
  - Given the sample CSV, running `import` populates `articles`, creates `folders` (“Unread”), and zero or more `tags`.
  - `instapapered_at` equals the converted CSV timestamp (UTC ISO8601).
  - Re-running `import` is idempotent (no duplicates by URL).

- **Fetch**
  - With no flags, processes up to **10** eligible articles.
  - Skips items where `synced_at` is set.
  - On success: `synced_at` set, `content_md` non-empty, `status_code=200`.
  - On failure: increments `failed_count`, sets `sync_failed_at`, records `status_code/text`.
  - Articles with `failed_count >= 5` are skipped until ≥1h elapsed.

- **Search**
  - `search "influencers"` finds the VICE example across title/content.
  - `search --field url "substack.com"` only matches URLs.

- **Export**
  - `export --id <id> --stdout` prints frontmatter + body.
  - `export-all --dir ./out` creates nested directories per folder `path_cache`, dedupes filenames, writes `.md` with correct frontmatter.
  - Adds `"instapaper"` tag to frontmatter alongside existing tags.

- **Schema**
  - Folders & tags use `title` as unique label.
  - Folder nesting can be manipulated via `folders mv/mkdir` and `path_cache` updates.

---

## 10) Test Plan (high level)

- Unit tests for:
  - CSV parsing (plain, JSON tags, empty arrays).
  - Timestamp conversion.
  - URL canonicalization.
  - Tag & folder upserts.
  - Fetcher selection logic (order, limit, retry thresholds).
  - Markdown conversion & frontmatter rendering.
  - Filename slugging & collision resolution.

- Integration tests:
  - End-to-end: import → fetch → export-all.
  - Simulated HTTP failures (timeouts, 404, 500).
  - Redirect chains; ensure `final_url` captured.
  - FTS queries return expected rows.

---

## 11) Observability

- Structured logs (JSON) with `article_id`, `url`, `event` (`fetch_ok`, `fetch_fail`, `import_row_skip`), `status_code`, `elapsed_ms`.
- `--log` to file; default stderr.
- `doctor` command: check foreign keys, run `PRAGMA integrity_check`, rebuild FTS, report counts by status.

---

## 12) Future Extensions (nice-to-have)

- Instapaper API ingestion (to get “mobilized text” directly).
- Watch mode (periodic fetch with jitter).
- Export to other formats (HTML, PDF, EPUB).
- Deduplicate by canonical URL hash.
- Simple TUI.

---

If you want, I can scaffold the repo (Cobra commands, migrations, and a minimal importer + fetcher) so you can `go run ./cmd/instapaper-cli import --csv ...` right away.
