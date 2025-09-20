CREATE VIRTUAL TABLE articles_fts USING fts5(
  url, title, content, folder, tags, content=''
);