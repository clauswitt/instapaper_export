CREATE TABLE rss_feeds (
  id INTEGER PRIMARY KEY,
  url TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_synced_at TEXT,
  active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE rss_feed_tags (
  feed_id INTEGER NOT NULL REFERENCES rss_feeds(id) ON DELETE CASCADE,
  tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (feed_id, tag_id)
);

CREATE INDEX idx_rss_feeds_active ON rss_feeds(active);
CREATE INDEX idx_rss_feeds_last_synced ON rss_feeds(last_synced_at);
