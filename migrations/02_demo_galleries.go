package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// demo_galleries is deliberately separate from the paid event schema. Each
// record is a tiny, anonymous one-photo gallery created by the landing-page
// demo and deleted after an hour by the retention job.
func init() {
	m.Register(func(app core.App) error {
		if coll, _ := app.FindCollectionByNameOrId("demo_galleries"); coll != nil {
			return nil
		}

		demos := core.NewBaseCollection("demo_galleries")
		demos.Fields.Add(&core.TextField{Name: "lang", Max: 5})
		demos.Fields.Add(&core.DateField{Name: "scanned_at"})
		demos.Fields.Add(&core.DateField{Name: "expires_at", Required: true})
		demos.Fields.Add(&core.FileField{
			Name:      "image",
			MaxSelect: 1,
			MaxSize:   15 << 20,
			Protected: true,
		})
		demos.Fields.Add(&core.FileField{
			Name:      "display",
			MaxSelect: 1,
			MaxSize:   15 << 20,
			Protected: true,
		})
		demos.Fields.Add(&core.BoolField{Name: "sample"})
		demos.Fields.Add(&core.TextField{Name: "original_name", Max: 200})
		demos.Fields.Add(&core.TextField{Name: "format", Max: 12})
		demos.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		demos.Fields.Add(&core.AutodateField{Name: "updated", OnUpdate: true})
		demos.AddIndex("idx_demo_galleries_expires", false, "expires_at", "")

		// The demo is available only through the purpose-built handlers. The
		// generic record API must not expose session metadata or file names.
		demos.ListRule = nil
		demos.ViewRule = nil
		demos.CreateRule = nil
		demos.UpdateRule = nil
		demos.DeleteRule = nil
		return app.Save(demos)
	}, func(app core.App) error {
		coll, _ := app.FindCollectionByNameOrId("demo_galleries")
		if coll == nil {
			return nil
		}
		return app.Delete(coll)
	})
}
