package app

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
)

// registerRetentionCleanup removes galleries once their promised one-year
// availability period ends. The job runs off-peak every day; route-level
// checks close access immediately at the exact anniversary, so there is no
// public gap while a gallery waits for the next cleanup pass.
func registerRetentionCleanup(app core.App) {
	app.Cron().MustAdd("qrpgExpiredGalleryCleanup", "15 3 * * *", func() {
		if err := cleanupExpiredGalleries(app); err != nil {
			log.Printf("expired gallery cleanup failed: %v", err)
		}
	})
}

func cleanupExpiredGalleries(app core.App) error {
	events, err := app.FindAllRecords("events")
	if err != nil {
		return err
	}
	for _, event := range events {
		if eventGalleryActive(event) {
			continue
		}
		if err := app.RunInTransaction(func(tx core.App) error {
			uploads, err := tx.FindRecordsByFilter("uploads", "event = {:eid}", "", 0, 0, dbxParams{"eid": event.Id})
			if err != nil {
				return err
			}
			for _, upload := range uploads {
				if err := tx.Delete(upload); err != nil {
					return err
				}
			}
			prompts, err := tx.FindRecordsByFilter("prompts", "event = {:eid}", "", 0, 0, dbxParams{"eid": event.Id})
			if err != nil {
				return err
			}
			for _, prompt := range prompts {
				if err := tx.Delete(prompt); err != nil {
					return err
				}
			}
			return tx.Delete(event)
		}); err != nil {
			return err
		}
	}
	return nil
}
