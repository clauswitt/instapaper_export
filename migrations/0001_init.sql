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
CREATE INDEX idx_articles_title ON articles(title);
CREATE UNIQUE INDEX idx_articles_url ON articles(url);