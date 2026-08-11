ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS media_id TEXT,
  ADD COLUMN IF NOT EXISTS media_mime_type TEXT,
  ADD COLUMN IF NOT EXISTS media_filename TEXT,
  ADD COLUMN IF NOT EXISTS media_size BIGINT,
  ADD COLUMN IF NOT EXISTS media_path TEXT;

CREATE INDEX IF NOT EXISTS messages_media_id_idx ON messages(media_id) WHERE media_id IS NOT NULL;
