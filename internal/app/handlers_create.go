package app

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// pendingEventCookie carries the two gallery details across the account
// creation round-trip. Internal upload-bucket details are never accepted from
// the browser.
const pendingEventCookie = "pending_event"

func handleCreateStart(e *core.RequestEvent) error {
	title, eventDate := readPendingCreate(e)
	return e.HTML(http.StatusOK, renderWithBase(e, "create", map[string]any{
		"EventTitle": title,
		"EventDate":  eventDate,
		"FormAction": "/create",
	}))
}

func handleCreateSubmit(e *core.RequestEvent) error {
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

	if e.Auth == nil {
		writePendingCreate(e, title, eventDate)
		redirect := url.QueryEscape("/create/finish")
		return redirectLocalised(e, http.StatusSeeOther, "/register?redirect="+redirect)
	}

	event, err := createEvent(e, title, eventDate)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	clearPendingCreate(e)
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

func handleCreateFinish(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	title, eventDate := readPendingCreate(e)
	if strings.TrimSpace(title) == "" {
		return redirectLocalised(e, http.StatusSeeOther, "/create")
	}

	event, err := createEvent(e, title, eventDate)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	clearPendingCreate(e)
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

func writePendingCreate(e *core.RequestEvent, title, eventDate string) {
	raw, _ := json.Marshal(map[string]string{
		"title":      title,
		"event_date": eventDate,
	})
	e.SetCookie(&http.Cookie{
		Name:     pendingEventCookie,
		Value:    base64.StdEncoding.EncodeToString(raw),
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func readPendingCreate(e *core.RequestEvent) (title, eventDate string) {
	cookie, err := e.Request.Cookie(pendingEventCookie)
	if err != nil || cookie.Value == "" {
		return "", ""
	}
	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "", ""
	}
	var pending map[string]string
	if err := json.Unmarshal(decoded, &pending); err != nil {
		return "", ""
	}
	return strings.TrimSpace(pending["title"]), strings.TrimSpace(pending["event_date"])
}

func clearPendingCreate(e *core.RequestEvent) {
	e.SetCookie(&http.Cookie{
		Name:     pendingEventCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
