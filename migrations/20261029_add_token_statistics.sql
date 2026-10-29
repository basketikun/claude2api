BEGIN IMMEDIATE;
ALTER TABLE api_logs ADD COLUMN thinking_tokens INTEGER;
CREATE TABLE usage_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    account TEXT,
    model TEXT,
    endpoint TEXT,
    success NUMERIC,
    duration_ms INTEGER,
    first_token_ms INTEGER,
    input_tokens INTEGER,
    output_tokens INTEGER,
    thinking_tokens INTEGER
);
CREATE INDEX idx_usage_records_created_at ON usage_records(created_at);
CREATE INDEX idx_usage_records_account ON usage_records(account);
CREATE INDEX idx_usage_records_model ON usage_records(model);
COMMIT;
