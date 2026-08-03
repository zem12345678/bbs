CREATE TABLE IF NOT EXISTS topic_polls (
  topic_id BIGINT PRIMARY KEY REFERENCES topics(id) ON DELETE CASCADE,
  multiple BOOLEAN NOT NULL DEFAULT FALSE,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS topic_poll_choices (
  topic_id BIGINT NOT NULL REFERENCES topic_polls(topic_id) ON DELETE CASCADE,
  choice_index SMALLINT NOT NULL,
  text VARCHAR(80) NOT NULL,
  votes_count BIGINT NOT NULL DEFAULT 0 CHECK (votes_count >= 0),
  PRIMARY KEY (topic_id, choice_index),
  CHECK (choice_index >= 0 AND choice_index < 10),
  CHECK (char_length(btrim(text)) BETWEEN 1 AND 80)
);

CREATE TABLE IF NOT EXISTS topic_poll_ballots (
  topic_id BIGINT NOT NULL REFERENCES topic_polls(topic_id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (topic_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_topic_poll_ballots_user_created
  ON topic_poll_ballots(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS topic_poll_ballot_choices (
  topic_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  choice_index SMALLINT NOT NULL,
  PRIMARY KEY (topic_id, user_id, choice_index),
  FOREIGN KEY (topic_id, user_id)
    REFERENCES topic_poll_ballots(topic_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (topic_id, choice_index)
    REFERENCES topic_poll_choices(topic_id, choice_index) ON DELETE CASCADE
);
