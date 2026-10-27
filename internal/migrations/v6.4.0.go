package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

func V6_4_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS campaign_sends (
			campaign_id   INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE ON UPDATE CASCADE,
			subscriber_id INTEGER NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE ON UPDATE CASCADE,
			created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			PRIMARY KEY (campaign_id, subscriber_id)
		);
		CREATE INDEX IF NOT EXISTS idx_camp_sends_sub_id ON campaign_sends(subscriber_id);
	`); err != nil {
		return err
	}

	return nil
}
