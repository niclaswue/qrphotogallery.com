package app

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestCleanupExpiredDemoGalleries(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo_galleries")
	if err != nil {
		t.Fatal(err)
	}

	expired := core.NewRecord(collection)
	expired.Set("expires_at", types.NowDateTime().Add(-time.Minute))
	image, err := filesystem.NewFileFromBytes([]byte("temporary demo bytes"), "temporary.jpg")
	if err != nil {
		t.Fatal(err)
	}
	expired.Set("image", image)
	if err := app.Save(expired); err != nil {
		t.Fatal(err)
	}
	expiredKey := expired.BaseFilesPath() + "/" + expired.GetString("image")

	active := core.NewRecord(collection)
	active.Set("expires_at", types.NowDateTime().Add(time.Hour))
	if err := app.Save(active); err != nil {
		t.Fatal(err)
	}

	if err := cleanupExpiredDemoGalleries(app); err != nil {
		t.Fatalf("cleanupExpiredDemoGalleries: %v", err)
	}
	if _, err := app.FindRecordById("demo_galleries", expired.Id); err == nil {
		t.Fatal("expired demo record still exists")
	}
	if _, err := app.FindRecordById("demo_galleries", active.Id); err != nil {
		t.Fatalf("active demo record was deleted: %v", err)
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		t.Fatal(err)
	}
	defer fsys.Close()
	exists, err := fsys.Exists(expiredKey)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expired demo image still exists in file storage")
	}
}
