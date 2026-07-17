package app

import (
	"errors"
	"log"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
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
	app.Cron().MustAdd("qrpgExpiredDemoCleanup", "*/10 * * * *", func() {
		if err := cleanupExpiredDemoGalleries(app); err != nil {
			log.Printf("expired demo gallery cleanup failed: %v", err)
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

// cleanupExpiredDemoGalleries runs frequently because demo uploads promise a
// one-hour lifetime. Deleting a record also deletes its PocketBase-managed
// files from local or S3 storage.
func cleanupExpiredDemoGalleries(app core.App) error {
	for {
		records, err := app.FindRecordsByFilter(
			"demo_galleries",
			"expires_at <= {:now}",
			"expires_at",
			500,
			0,
			dbxParams{"now": types.NowDateTime()},
		)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		for _, record := range records {
			if err := deleteExpiredDemoGallery(app, record); err != nil {
				return err
			}
		}
	}
}

func deleteExpiredDemoGallery(app core.App, record *core.Record) error {
	// Remove the whole opaque record prefix explicitly. PocketBase's full app
	// also has a record-delete file hook, but doing it here keeps retention
	// self-contained and makes storage cleanup reliable in stripped-down test
	// apps as well. If storage fails, retain the DB row so the next cron run can
	// retry instead of orphaning an unreachable object.
	fsys, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	deleteErrors := fsys.DeletePrefix(record.BaseFilesPath() + "/")
	closeErr := fsys.Close()
	if len(deleteErrors) > 0 || closeErr != nil {
		joined := append(deleteErrors, closeErr)
		return errors.Join(joined...)
	}
	return app.Delete(record)
}
