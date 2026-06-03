package tracker

import (
	"errors"
	"strings"
)

// Event represents a Matomo tracking event to be published to NATS
type Event struct {
	// Required
	SiteID string `json:"idsite"`
	Rec    string `json:"rec,omitempty"`

	// Recommended
	ActionName string `json:"action_name,omitempty"`
	URL        string `json:"url,omitempty"`
	VisitorID  string `json:"_id,omitempty"`
	Rand       string `json:"rand,omitempty"`
	APIVersion string `json:"apiv,omitempty"`

	// User info
	Referrer    string `json:"urlref,omitempty"`
	Resolution  string `json:"res,omitempty"`
	Hour        string `json:"h,omitempty"`
	Minute      string `json:"m,omitempty"`
	Second      string `json:"s,omitempty"`
	UserAgent   string `json:"ua,omitempty"`
	ClientHints string `json:"uadata,omitempty"`
	Language    string `json:"lang,omitempty"`
	UserID      string `json:"uid,omitempty"`
	VisitorUUID string `json:"cid,omitempty"`
	NewVisit    string `json:"new_visit,omitempty"`
	CustomVars  string `json:"_cvar,omitempty"`
	Cookie      string `json:"cookie,omitempty"`

	// Plugins (flash, java, director, quicktime, realplayer, pdf, wma, gears, silverlight)
	Plugins map[string]string `json:"plugins,omitempty"`

	// Acquisition
	CampaignName    string `json:"_rcn,omitempty"`
	CampaignKeyword string `json:"_rck,omitempty"`
	// AttributionReferrer (_ref) is the referrer the visit is attributed to,
	// stored in the visitor's first-party cookie — distinct from urlref, which is
	// the immediate referrer of this request. AttributionReferrerTime (_refts) is
	// that attribution referrer's timestamp.
	AttributionReferrer     string `json:"_ref,omitempty"`
	AttributionReferrerTime string `json:"_refts,omitempty"`

	// Action info
	PageCustomVars string `json:"cvar,omitempty"`
	Outlink        string `json:"link,omitempty"`
	Download       string `json:"download,omitempty"`
	SearchKeyword  string `json:"search,omitempty"`
	SearchCategory string `json:"search_cat,omitempty"`
	SearchCount    string `json:"search_count,omitempty"`
	PageViewID     string `json:"pv_id,omitempty"`
	GoalID         string `json:"idgoal,omitempty"`
	Revenue        string `json:"revenue,omitempty"`
	Charset        string `json:"cs,omitempty"`
	CustomAction   string `json:"ca,omitempty"`

	// Page Performance
	PerfNetwork       string `json:"pf_net,omitempty"`
	PerfServer        string `json:"pf_srv,omitempty"`
	PerfTransfer      string `json:"pf_tfr,omitempty"`
	PerfDOMProcessing string `json:"pf_dm1,omitempty"`
	PerfDOMCompletion string `json:"pf_dm2,omitempty"`
	PerfOnLoad        string `json:"pf_onl,omitempty"`

	// Event Tracking
	EventCategory string `json:"e_c,omitempty"`
	EventAction   string `json:"e_a,omitempty"`
	EventName     string `json:"e_n,omitempty"`
	EventValue    string `json:"e_v,omitempty"`

	// Content Tracking
	ContentName        string `json:"c_n,omitempty"`
	ContentPiece       string `json:"c_p,omitempty"`
	ContentTarget      string `json:"c_t,omitempty"`
	ContentInteraction string `json:"c_i,omitempty"`

	// Ecommerce
	EcommerceOrderID  string `json:"ec_id,omitempty"`
	EcommerceItems    string `json:"ec_items,omitempty"`
	EcommerceSubtotal string `json:"ec_st,omitempty"`
	EcommerceTax      string `json:"ec_tx,omitempty"`
	EcommerceShipping string `json:"ec_sh,omitempty"`
	EcommerceDiscount string `json:"ec_dt,omitempty"`
	ProductCategory   string `json:"_pkc,omitempty"`
	ProductPrice      string `json:"_pkp,omitempty"`
	ProductSKU        string `json:"_pks,omitempty"`
	ProductName       string `json:"_pkn,omitempty"`

	// Dimensions holds every dimensionN param verbatim. A dimension's scope
	// (visit vs action) is fixed server-side in Matomo and is not derivable from
	// the request, so this stateless forwarder does not infer it from ca — the
	// downstream consumer that owns the dimension config assigns scope.
	Dimensions map[string]string `json:"dimensions,omitempty"`

	// Response control
	SendImage string `json:"send_image,omitempty"`
	Ping      string `json:"ping,omitempty"`

	// Bot tracking
	RecMode          string `json:"recMode,omitempty"`
	Bots             string `json:"bots,omitempty"`
	HTTPStatus       string `json:"http_status,omitempty"`
	BytesTransferred string `json:"bw_bytes,omitempty"`
	BotSource        string `json:"source,omitempty"`

	// Auth/override (requires token_auth)
	TokenAuth    string `json:"token_auth,omitempty"`
	OverrideIP   string `json:"cip,omitempty"`
	OverrideTime string `json:"cdt,omitempty"`
	// TimestampOffset (cdo) is a seconds offset subtracted from the effective
	// event time (Matomo: cdt - abs(cdo)); used by offline/queued SDKs to backdate
	// events. Forwarded so a time-bucketing consumer can apply it.
	TimestampOffset string `json:"cdo,omitempty"`
	Country         string `json:"country,omitempty"`
	Region          string `json:"region,omitempty"`
	City            string `json:"city,omitempty"`
	Latitude        string `json:"lat,omitempty"`
	Longitude       string `json:"long,omitempty"`

	// Debug
	Debug string `json:"debug,omitempty"`

	// Action type for NATS routing
	ActionType string `json:"action_type,omitempty"`

	// Bulk tracking
	BulkRequests []string `json:"requests,omitempty"`

	// Extra holds request parameters the parser did not place in a named field:
	// unrecognized params, plus recognized-but-invalid visitor IDs demoted here
	// (e.g. a malformed _id/cid). Nothing the client sends is dropped. Empty when
	// every parameter was recognized and valid.
	Extra map[string]string `json:"extra,omitempty"`
}

// handledParams lists every parameter key ParseEvent maps to a struct field
// (including the plugin-flag keys, seeded in init). Keys NOT in this set, and
// not prefixed with "dimension", are forwarded verbatim in Event.Extra. The
// TestParseEvent_HandledParamsCoverAllMappedKeys drift guard reflects over the
// Event struct's json tags and fails if an input field's key is missing here,
// so a forgotten entry can't silently double-publish into Extra.
var handledParams = map[string]struct{}{
	"idsite": {}, "rec": {}, "action_name": {}, "url": {}, "_id": {},
	"rand": {}, "apiv": {}, "urlref": {}, "res": {}, "h": {}, "m": {}, "s": {},
	"ua": {}, "uadata": {}, "lang": {}, "uid": {}, "cid": {}, "new_visit": {},
	"_cvar": {}, "cookie": {}, "_rcn": {}, "_rck": {}, "_ref": {}, "_refts": {},
	"cdo": {}, "cvar": {}, "link": {},
	"download": {}, "search": {}, "search_cat": {}, "search_count": {},
	"pv_id": {}, "idgoal": {}, "revenue": {}, "cs": {}, "ca": {},
	"pf_net": {}, "pf_srv": {}, "pf_tfr": {}, "pf_dm1": {}, "pf_dm2": {}, "pf_onl": {},
	"e_c": {}, "e_a": {}, "e_n": {}, "e_v": {},
	"c_n": {}, "c_p": {}, "c_t": {}, "c_i": {},
	"ec_id": {}, "ec_items": {}, "ec_st": {}, "ec_tx": {}, "ec_sh": {}, "ec_dt": {},
	"_pkc": {}, "_pkp": {}, "_pks": {}, "_pkn": {},
	"send_image": {}, "ping": {}, "recMode": {}, "bots": {}, "http_status": {},
	"bw_bytes": {}, "source": {}, "token_auth": {}, "cip": {}, "cdt": {},
	"country": {}, "region": {}, "city": {}, "lat": {}, "long": {}, "debug": {},
}

// pluginKeys are the Matomo plugin-availability flags, mapped into Event.Plugins.
// They are the single source of truth for both the plugin-parsing loop and the
// handledParams set (seeded in init), so the two can't drift.
var pluginKeys = []string{"fla", "java", "dir", "qt", "realp", "pdf", "wma", "gears", "ag"}

func init() {
	for _, k := range pluginKeys {
		handledParams[k] = struct{}{}
	}
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

	e := &Event{
		SiteID:                  siteID,
		Rec:                     rec,
		ActionName:              params["action_name"],
		URL:                     params["url"],
		VisitorID:               params["_id"],
		Rand:                    params["rand"],
		APIVersion:              params["apiv"],
		Referrer:                params["urlref"],
		Resolution:              params["res"],
		Hour:                    params["h"],
		Minute:                  params["m"],
		Second:                  params["s"],
		UserAgent:               params["ua"],
		ClientHints:             params["uadata"],
		Language:                params["lang"],
		UserID:                  params["uid"],
		VisitorUUID:             params["cid"],
		NewVisit:                params["new_visit"],
		CustomVars:              params["_cvar"],
		Cookie:                  params["cookie"],
		CampaignName:            params["_rcn"],
		CampaignKeyword:         params["_rck"],
		AttributionReferrer:     params["_ref"],
		AttributionReferrerTime: params["_refts"],
		PageCustomVars:          params["cvar"],
		Outlink:                 params["link"],
		Download:                params["download"],
		SearchKeyword:           params["search"],
		SearchCategory:          params["search_cat"],
		SearchCount:             params["search_count"],
		PageViewID:              params["pv_id"],
		GoalID:                  params["idgoal"],
		Revenue:                 params["revenue"],
		Charset:                 params["cs"],
		CustomAction:            params["ca"],
		PerfNetwork:             params["pf_net"],
		PerfServer:              params["pf_srv"],
		PerfTransfer:            params["pf_tfr"],
		PerfDOMProcessing:       params["pf_dm1"],
		PerfDOMCompletion:       params["pf_dm2"],
		PerfOnLoad:              params["pf_onl"],
		EventCategory:           params["e_c"],
		EventAction:             params["e_a"],
		EventName:               params["e_n"],
		EventValue:              params["e_v"],
		ContentName:             params["c_n"],
		ContentPiece:            params["c_p"],
		ContentTarget:           params["c_t"],
		ContentInteraction:      params["c_i"],
		EcommerceOrderID:        params["ec_id"],
		EcommerceItems:          params["ec_items"],
		EcommerceSubtotal:       params["ec_st"],
		EcommerceTax:            params["ec_tx"],
		EcommerceShipping:       params["ec_sh"],
		EcommerceDiscount:       params["ec_dt"],
		ProductCategory:         params["_pkc"],
		ProductPrice:            params["_pkp"],
		ProductSKU:              params["_pks"],
		ProductName:             params["_pkn"],
		SendImage:               params["send_image"],
		Ping:                    params["ping"],
		RecMode:                 params["recMode"],
		Bots:                    params["bots"],
		HTTPStatus:              params["http_status"],
		BytesTransferred:        params["bw_bytes"],
		BotSource:               params["source"],
		TokenAuth:               params["token_auth"],
		OverrideIP:              params["cip"],
		OverrideTime:            params["cdt"],
		TimestampOffset:         params["cdo"],
		Country:                 params["country"],
		Region:                  params["region"],
		City:                    params["city"],
		Latitude:                params["lat"],
		Longitude:               params["long"],
		Debug:                   params["debug"],
	}

	// Parse plugin flags
	e.Plugins = make(map[string]string)
	for _, plugin := range pluginKeys {
		if v := params[plugin]; v != "" {
			e.Plugins[plugin] = v
		}
	}

	// Classify the parameters the struct literal didn't already name: dimensionN
	// keys into Dimensions, everything else unrecognized into Extra. One pass,
	// both maps allocated lazily so the common all-recognized request allocates
	// neither.
	e.Dimensions, e.Extra = classifyParams(params)

	// Validate visitor IDs: keep valid 16-hex values (lowercased) and demote
	// invalid ones into Extra so a bad ID is forwarded, not trusted as a visitor
	// key. Matches Matomo's "ignore invalid ID, still track" behavior.
	e.validateID(&e.VisitorID, "_id")
	e.validateID(&e.VisitorUUID, "cid")

	e.ActionType = detectActionType(params)

	return e, nil
}

// classifyParams routes each parameter not already mapped to a named struct
// field into one of two buckets in a single pass: dimensionN keys into dims
// (scope assigned downstream — see Event.Dimensions), and any other key the
// parser doesn't recognize into extra (so nothing the client sends is dropped).
// Both maps stay nil until first use, so the common all-recognized request
// allocates neither and the omitempty JSON tags drop both fields.
func classifyParams(params map[string]string) (dims, extra map[string]string) {
	for key, val := range params {
		switch {
		case strings.HasPrefix(key, "dimension"):
			dims = stashExtra(dims, key, val)
		default:
			if _, known := handledParams[key]; known {
				continue
			}
			extra = stashExtra(extra, key, val)
		}
	}
	return dims, extra
}

// stashExtra adds k=v to m, allocating m if nil, and returns it. It is the
// single "ensure map, insert" primitive used to build Event.Extra and
// Event.Dimensions, keeping both nil until they actually hold something.
func stashExtra(m map[string]string, k, v string) map[string]string {
	if m == nil {
		m = make(map[string]string)
	}
	m[k] = v
	return m
}

// validateID normalizes a 16-hex visitor-ID field (*field) in place: a valid
// value is lowercased; an invalid one is moved to Extra and the field cleared,
// so a malformed ID is forwarded but not trusted as a visitor key. Empty is
// left as-is (a missing visitor ID is normal).
func (e *Event) validateID(field *string, key string) {
	if *field == "" {
		return
	}
	if norm, ok := normalizeVisitorID(*field); ok {
		*field = norm
	} else {
		e.Extra = stashExtra(e.Extra, key, *field)
		*field = ""
	}
}

// normalizeVisitorID returns ok=true when v is a 16-character hexadecimal
// string, along with v lowercased. The common case (already lowercase) returns
// v unchanged with no allocation; a copy is made only when an uppercase hex
// digit must be lowered.
func normalizeVisitorID(v string) (string, bool) {
	if len(v) != 16 {
		return "", false
	}
	needsLower := false
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
			// valid digit / lowercase hex
		case c >= 'A' && c <= 'F':
			needsLower = true
		default:
			return "", false
		}
	}
	if !needsLower {
		return v, true // already valid lowercase — no allocation
	}
	b := []byte(v)
	for i := range b {
		if c := b[i]; c >= 'A' && c <= 'F' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b), true
}

// detectActionType determines what kind of action is being tracked
func detectActionType(params map[string]string) string {
	switch {
	case params["ping"] == "1":
		return "heartbeat"
	case params["e_c"] != "" && params["e_a"] != "":
		return "event"
	case params["idgoal"] != "" && params["idgoal"] != "0":
		return "goal"
	case params["idgoal"] == "0":
		return "ecommerce"
	case params["search"] != "":
		return "search"
	case params["link"] != "":
		return "outlink"
	case params["download"] != "":
		return "download"
	case params["c_n"] != "" && params["c_i"] != "":
		return "content_interaction"
	case params["c_n"] != "":
		return "content_impression"
	default:
		return "pageview"
	}
}
