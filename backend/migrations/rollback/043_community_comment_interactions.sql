-- Roll back only the community comment interaction columns and relation.
DROP TABLE IF EXISTS comment_likes;

ALTER TABLE comments
  DROP CHECK chk_comment_liked_count,
  DROP KEY idx_comment_thread_page,
  DROP COLUMN liked_count,
  DROP COLUMN reply_to_user_id,
  DROP COLUMN reply_to_comment_id,
  DROP COLUMN parent_comment_id;
