-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS set_updated_at ON lambda_scaling_policies;
DROP FUNCTION IF EXISTS update_updated_at_column;

DROP TABLE IF EXISTS lambda_scaling_policies;

-- +goose StatementEnd