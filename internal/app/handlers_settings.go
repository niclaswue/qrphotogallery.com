package app

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

func handleEditGallery(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	return e.HTML(http.StatusOK, renderWithBase(e, "create", map[string]any{
		"EventTitle": event.GetString("title"),
		"EventDate":  event.GetString("event_date"),
		"FormAction": "/edit/" + event.Id,
		"EditMode":   true,
		"EventID":    event.Id,
	}))
}

func handleEditGallerySubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	data := struct {
		Title     string `json:"title" form:"title"`
		EventDate string `json:"event_date" form:"event_date"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.invalid_form")
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.missing_gallery_name")
	}
	if len([]rune(title)) > 120 {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.gallery_name_too_long")
	}
	eventDate := strings.TrimSpace(data.EventDate)
	if eventDate != "" && !isValidDate(eventDate) {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.invalid_date")
	}

	event.Set("title", title)
	event.Set("event_date", eventDate)
	if err := e.App.Save(event); err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// Commercial accounts can ask for uploader names and decide whether guests
// may download the shared ZIP.
func handleEventSettingsSubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	if !isPremiumTier(getUserTier(e.Auth).Name) {
		return renderHTMLErrorKeys(e, http.StatusForbidden, "error.title.commercial_feature", "error.message.commercial_feature")
	}

	data := struct {
		DisableGuestDownload string `json:"disable_guest_download" form:"disable_guest_download"`
		CollectGuestName     string `json:"collect_guest_name" form:"collect_guest_name"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.invalid_form")
	}
	event.Set("disable_guest_download", isCheckboxOn(data.DisableGuestDownload))
	event.Set("collect_guest_name", isCheckboxOn(data.CollectGuestName))
	if err := e.App.Save(event); err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

func handleEventLangSubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	if err := e.Request.ParseForm(); err != nil {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.invalid_form")
	}
	lang := e.Request.FormValue("lang")
	if !i18n.IsSupported(lang) {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.unsupported_language")
	}
	event.Set("lang", lang)
	if err := e.App.Save(event); err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

func handleDeleteEvent(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	err = e.App.RunInTransaction(func(tx core.App) error {
		prompts, err := tx.FindRecordsByFilter("prompts", "event = {:eid}", "", 0, 0, dbxParams{"eid": event.Id})
		if err != nil {
			return err
		}
		for _, prompt := range prompts {
			if err := deletePromptUploads(tx, prompt.Id); err != nil {
				return err
			}
			if err := tx.Delete(prompt); err != nil {
				return err
			}
		}
		return tx.Delete(event)
	})
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	return redirectLocalised(e, http.StatusSeeOther, "/overview")
}

func isCheckboxOn(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "1", "true", "yes":
		return true
	default:
		return false
	}
}

func handleGetUserTier(e *core.RequestEvent) error {
	if e.Auth == nil {
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	return e.JSON(http.StatusOK, map[string]string{"tier": getUserTier(e.Auth).Name})
}
