-- Add obsolete column to articles table
-- This allows marking articles as obsolete without deleting them,
-- preventing re-import and unnecessary fetch attempts

ALTER TABLE articles ADD COLUMN obsolete BOOLEAN NOT NULL DEFAULT FALSE;

-- Add index for efficient filtering
CREATE INDEX idx_articles_obsolete ON articles(obsolete);