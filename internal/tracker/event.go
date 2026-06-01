package tracker

import (
	"errors"
)

// Event represents a Matomo tracking event to be published to NATS
type Event struct {
	SiteID      string `json:"idsite"`
	URL         string `json:"url,omitempty"`
	ActionName  string `json:"action_name,omitempty"`
	VisitorID   string `json:"_id,omitempty"`
	UserID      string `json:"uid,omitempty"`
	VisitorUUID string `json:"cid,omitempty"`
	Resolution  string `json:"res,omitempty"`
	Language    string `json:"lang,omitempty"`
	UserAgent   string `json:"ua,omitempty"`
	Referrer    string `json:"urlref,omitempty"`
}

// ParseEvent validates and parses tracking parameters into an Event
func ParseEvent(params map[string]string) (*Event, error) {
	siteID, ok := params["idsite"]
	if !ok || siteID == "" {
		return nil, errors.New("missing required parameter: idsite")
	}

	rec, ok := params["rec"]
	if !ok || rec == "" {
		return nil, errors.New("missing required parameter: rec")
	}

	return &Event{
		SiteID:      siteID,
		URL:         params["url"],
		ActionName:  params["action_name"],
		VisitorID:   params["_id"],
		UserID:      params["uid"],
		VisitorUUID: params["cid"],
		Resolution:  params["res"],
		Language:    params["lang"],
		UserAgent:   params["ua"],
		Referrer:    params["urlref"],
	}, nil
}
