package terabox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

var errQRPending = errors.New("qr login pending")

// errQRSessionDead marks a poll reply saying the stored QR session is gone
// (invalid or expired uuid). Polling further is pointless; the caller should
// start a fresh QR flow instead of failing hard and bricking the storage.
var errQRSessionDead = errors.New("qr session dead")

// qrLoginPageURL optionally overrides the QR login page (unit tests set it).
// Empty means "derive from the driver's resolved base URL" — TeraBox serves
// the same QR page on every alias and picks the region dynamically via the
// Url-Domain-Prefix header, so a hardcoded host is wrong.
var qrLoginPageURL = ""

// qrPageURL resolves the QR login page: the test override when set, otherwise
// the page on the driver's current base URL (www.terabox.com by default, or
// the region domain chosen by the request() -6 handler).
func (d *Terabox) qrPageURL() string {
	if qrLoginPageURL != "" {
		return qrLoginPageURL
	}
	if d.base_url != "" {
		return d.base_url + "/ai/index"
	}
	return "https://www.terabox.com/ai/index"
}

const (
	// QR login flow parameters — mirrors bin/terabox-login from
	// github.com/mill-master/rclone-terabox.
	//
	// qrLoginTimeout is deliberately much shorter than the reference's 180s:
	// here the polling runs inside a storage save HTTP request, and real-world
	// proxies kill those at ~100-120s (observed live 2026-11-24: the request
	// died with "context canceled" at exactly 2m0s). The window must end with
	// time to spare so loginByQRCodeAt answers with a fresh QR page instead
	// of an opaque canceled request.
	qrLoginTimeout = 90 * time.Second
	qrPollInterval = 2 * time.Second
	// The /passport/qrcode/login endpoint long-polls accepted requests
	// (~26-35s) before replying with 39/"login timeout"; the request
	// timeout must cover that window (the reference uses 35s).
	qrPollRequestTimeout = 35 * time.Second
	qrPendingCode        = 39
	// qrStateMaxAge is how long a stored QR stays worth polling. The QR image
	// is only scannable for a few minutes (the TeraBox app rejects old codes as
	// expired — the reference CLI gives a single QR a 180s window), so a stored
	// QR older than this is reissued instead of polled.
	qrStateMaxAge = 150 * time.Second
	// TeraBox codes observed live (2026-11-24): the stored QR session is dead.
	qrSessionInvalidCode = 36 // "invalid uuid"
	qrSessionExpiredCode = 37 // "uuid expire"
	qrLoginUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"
	qrLoginLang = "en"
)

// qrLoginPageTemplate renders the QR page shown to the user. The QR page
// carries the login state in `data-state` (JSON, HTML-escaped) so the
// frontend can start polling after the user scans and re-saves the storage.
const qrLoginPageTemplate = `<body data-state="%s"><br><img src="%s"/><br>Scan the QR code with the TeraBox app, then confirm the login on the phone.</body>`

// qrLoginState is the login-session state exchanged with /passport/qrcode/login.
type qrLoginState struct {
	Browserid   string `json:"browserid"`
	Client      string `json:"client"`       // "web"
	Clientfrom  string `json:"clientfrom"`   // "h5"
	Lang        string `json:"lang"`         // "en"
	PassVersion string `json:"pass_version"` // "2.8"
	PCFToken    string `json:"pcftoken"`
	RegSource   string `json:"reg_source"` // "web"
	Seq         int64  `json:"seq"`
	UUID        string `json:"uuid"`
	PageURL     string `json:"page_url"`         // final URL after redirects
	QRImage     string `json:"qrcode,omitempty"` // QR image data URI (for display on re-edit)
	// CreatedAt is when the QR was issued (unix seconds). The QR image is only
	// scannable for a short while (the TeraBox app rejects old codes as
	// expired), much shorter than the uuid session lives — polling a stale QR
	// can never receive a confirmation.
	CreatedAt int64 `json:"created_at,omitempty"`
	// v/vcode are verification-challenge tokens captured from a poll reply and
	// echoed back on the next poll (mirrors the reference implementation).
	V     string `json:"v,omitempty"`
	VCode string `json:"vcode,omitempty"`
}

func (s *qrLoginState) form() map[string]string {
	form := map[string]string{
		"browserid":    s.Browserid,
		"client":       s.Client,
		"clientfrom":   s.Clientfrom,
		"lang":         s.Lang,
		"pass_version": s.PassVersion,
		"pcftoken":     s.PCFToken,
		"reg_source":   s.RegSource,
		"seq":          strconv.FormatInt(s.Seq, 10),
		"uuid":         s.UUID,
	}
	// Echo verification-challenge tokens when the account needed one.
	if s.V != "" {
		form["v"] = s.V
	}
	if s.VCode != "" {
		form["vcode"] = s.VCode
	}
	return form
}

// captureChallenge stores v/vcode verification tokens from a poll reply so
// the next poll echoes them back (mirrors bin/terabox-login).
func (s *qrLoginState) captureChallenge(r *qrLoginStatusResp) {
	if r.V != "" {
		s.V = r.V
	} else if r.Data.V != "" {
		s.V = r.Data.V
	}
	if r.VCode != "" {
		s.VCode = r.VCode
	} else if r.Data.VCode != "" {
		s.VCode = r.Data.VCode
	}
}

type qrLoginGetResp struct {
	Code  int `json:"code"`
	Errno int `json:"errno"`
	Data  struct {
		UUID string `json:"uuid"`
		QR   string `json:"qrcode"` // data:image/png;base64,...
		Seq  int64  `json:"seq"`
	} `json:"data"`
}

func (r *qrLoginGetResp) statusCode() int {
	if r.Code != 0 {
		return r.Code
	}
	return r.Errno
}

type qrLoginStatusResp struct {
	Code  int `json:"code"`
	Errno int `json:"errno"`
	// v/vcode may appear at the top level or inside data when the account
	// needs extra verification after scanning.
	V     string `json:"v"`
	VCode string `json:"vcode"`
	Data  struct {
		NDUs  string `json:"ndus"`
		UName string `json:"uname"`
		V     string `json:"v"`
		VCode string `json:"vcode"`
	} `json:"data"`
}

func (r *qrLoginStatusResp) statusCode() int {
	if r.Code != 0 {
		return r.Code
	}
	return r.Errno
}

// qrPageHTML renders the QR page shown to the user (state in `data-state`,
// QR image as the src) so the frontend can display/poll it.
func (d *Terabox) qrPageHTML(state *qrLoginState) (string, error) {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(qrLoginPageTemplate, html.EscapeString(string(stateJSON)), state.QRImage), nil
}

// qrVerifyError returns the "keep waiting" error carrying the CURRENT QR
// session's page. Reusing the session is deliberate: TeraBox QR sessions live
// 20+ minutes, and issuing a fresh QR on every save window abandoned sessions
// users were still confirming in the phone app (live 2026-11-24: app said
// success while the poll kept asking about a newer uuid forever).
func (d *Terabox) qrVerifyError(state *qrLoginState) error {
	qrPage, err := d.qrPageHTML(state)
	if err != nil {
		return err
	}
	return fmt.Errorf("need verify: \n%s", qrPage)
}

// startQRLogin obtains a fresh QR code from TeraBox.
func (d *Terabox) startQRLogin(ctx context.Context) (*qrLoginState, string, error) {
	jar, _ := cookiejar.New(nil)
	client := resty.New().SetCookieJar(jar).SetTimeout(15 * time.Second)

	// 1) GET login page -> browserid cookie + pcftoken in HTML
	resp, err := client.R().
		SetHeader("User-Agent", qrLoginUserAgent).
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		SetHeader("Accept-Language", "en-US,en;q=0.9").
		SetContext(ctx).
		Get(d.qrPageURL())
	if err != nil {
		return nil, "", err
	}
	pageURL := resp.RawResponse.Request.URL.String()
	pageHTML := resp.String()

	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, "", err
	}
	origin := u.Scheme + "://" + u.Host

	browserid := cookieFromJar(jar, u, "browserid")
	if browserid == "" {
		return nil, "", fmt.Errorf("TeraBox did not set a browserid cookie on %s", pageURL)
	}
	pcftoken := getStrBetween(pageHTML, `"pcftoken":"`, `"`)
	if pcftoken == "" {
		return nil, "", fmt.Errorf("pcftoken not found in %s", pageURL)
	}

	// 2) POST /passport/qrcode/get
	var res qrLoginGetResp
	qrResp, err := client.R().
		SetResult(&res).
		SetFormData(map[string]string{
			"browserid":    browserid,
			"client":       "web",
			"clientfrom":   "h5",
			"lang":         qrLoginLang,
			"pass_version": "2.8",
			"pcftoken":     pcftoken,
		}).
		SetHeader("Origin", origin).
		SetHeader("Referer", pageURL).
		SetHeader("X-Requested-With", "XMLHttpRequest").
		SetQueryParam("t", strconv.FormatInt(time.Now().UnixMilli(), 10)).
		SetContext(ctx).
		Post(origin + "/passport/qrcode/get")
	if err != nil {
		return nil, "", err
	}
	if code := res.statusCode(); code != 0 || res.Data.UUID == "" || res.Data.QR == "" || res.Data.Seq == 0 {
		return nil, "", fmt.Errorf("QR request failed: %s", truncate(qrResp.String(), 300))
	}

	state := &qrLoginState{
		Browserid:   browserid,
		Client:      "web",
		Clientfrom:  "h5",
		Lang:        qrLoginLang,
		PassVersion: "2.8",
		PCFToken:    pcftoken,
		RegSource:   "web",
		Seq:         res.Data.Seq,
		UUID:        res.Data.UUID,
		PageURL:     pageURL,
		QRImage:     res.Data.QR,
		CreatedAt:   time.Now().Unix(),
	}
	qrPage, err := d.qrPageHTML(state)
	if err != nil {
		return nil, "", err
	}
	return state, qrPage, nil
}

// pollQRStatus checks /passport/qrcode/login; returns the ndus token when the user confirmed in the app.
func (d *Terabox) pollQRStatus(ctx context.Context, state *qrLoginState) (string, string, error) {
	jar, _ := cookiejar.New(nil)
	// The passport endpoint rejects cookieless, headerless requests with
	// code 6 "System error" (verified against the real endpoint), so seed
	// the jar with the browserid cookie of this QR session.
	if u, err := url.Parse(state.PageURL); err == nil && u.Host != "" {
		jar.SetCookies(u, []*http.Cookie{{Name: "browserid", Value: state.Browserid, Path: "/"}})
	}
	// Accepted requests are long-polled (~26-35s) before the endpoint answers.
	client := resty.New().SetCookieJar(jar).SetTimeout(qrPollRequestTimeout)

	var res qrLoginStatusResp
	resp, err := client.R().
		SetHeader("User-Agent", qrLoginUserAgent).
		SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Accept-Language", "en-US,en;q=0.9").
		SetHeader("Origin", originOf(state.PageURL)).
		SetHeader("Referer", state.PageURL).
		SetHeader("X-Requested-With", "XMLHttpRequest").
		SetResult(&res).
		SetFormData(state.form()).
		SetQueryParam("t", strconv.FormatInt(time.Now().UnixMilli(), 10)).
		SetContext(ctx).
		Post(originOf(state.PageURL) + "/passport/qrcode/login")
	if err != nil {
		return "", "", err
	}

	code := res.statusCode()
	if code != qrPendingCode {
		// Decisive evidence for the "app confirmed but poll never sees it" bug:
		// log every non-pending poll outcome (code + whether the token arrived).
		log.Infof("TeraBox QR poll: code=%d ndus_present=%t uname=%q", code, res.Data.NDUs != "", res.Data.UName)
	}
	if code == 0 {
		// Scanned: capture v/vcode challenge tokens for the next poll.
		state.captureChallenge(&res)
		ndus := res.Data.NDUs
		if ndus == "" {
			if u, _ := url.Parse(state.PageURL); u != nil {
				ndus = cookieFromJar(jar, u, "ndus")
			}
		}
		if ndus == "" {
			return "", "", errQRPending
		}
		return ndus, res.Data.UName, nil
	}
	if code == qrPendingCode {
		return "", "", errQRPending
	}
	if code == qrSessionInvalidCode || code == qrSessionExpiredCode {
		return "", "", fmt.Errorf("%w: %s", errQRSessionDead, truncate(resp.String(), 300))
	}
	return "", "", fmt.Errorf("QR login failed: %s", truncate(resp.String(), 300))
}

func (d *Terabox) loginByQRCodeAt(ctx context.Context, timeout, pollInterval time.Duration) error {
	state := d.QRState
	if state == nil {
		s, _, err := d.startQRLogin(ctx)
		if err != nil {
			return err
		}
		d.QRState = s
		state = s
	} else if qrStateExpired(state) {
		// The stored QR is older than TeraBox keeps a code scannable: reissue
		// before polling, otherwise the phone app rejects it as expired and no
		// confirmation can ever arrive for this session. Answer fast with the
		// fresh QR page so the UI updates immediately.
		s, _, err := d.startQRLogin(ctx)
		if err != nil {
			return err
		}
		d.QRState = s
		return d.qrVerifyError(s)
	}

	deadline := time.Now().Add(timeout)
	for {
		ndus, uname, err := d.pollQRStatus(ctx, state)
		if err == nil {
			d.Cookie = "ndus=" + ndus
			d.QRState = nil
			// Keep d.base_url on the resolved official/region domain (set by
			// Init or the Url-Domain-Prefix -6 handler). Do NOT pin it to the
			// QR page origin: the ndus cookie is account-scoped, and pinning
			// to a hardcoded alias breaks TeraBox's dynamic endpoint.
			log.Debugln("TeraBox QR login: token disimpan, akun:", uname)
			return nil
		}
		if errors.Is(err, errQRSessionDead) {
			// The stored QR session is invalid or expired: start a fresh QR
			// flow so the user gets a new code instead of a bricked storage.
			return d.startQRFlow(ctx)
		}
		if err != errQRPending {
			return err
		}
		// Don't start a poll that could still be long-polling (~35s) when the
		// window ends: it would overshoot the request's lifetime and the save
		// would die as an opaque "context canceled" instead of answering.
		// Keep the CURRENT session: the QR stays valid for 20+ minutes, and a
		// fresh QR here would abandon the session the user is confirming in
		// the app. A dead session is detected via errQRSessionDead instead.
		if time.Now().Add(qrPollRequestTimeout + pollInterval).After(deadline) {
			return d.qrVerifyError(state)
		}
		time.Sleep(pollInterval)
	}
}

// startQRFlow starts a new QR login flow and returns the QR page error so the
// frontend can display the QR code; the state is stored on the driver so the
// next save can poll (loginByQRCodeAt).
func (d *Terabox) startQRFlow(ctx context.Context) error {
	state, _, err := d.startQRLogin(ctx)
	if err != nil {
		return err
	}
	d.QRState = state
	return d.qrVerifyError(state)
}

// qrStateExpired reports whether a stored QR is too old to be scanned: the QR
// image (not just the uuid session) has a short lifetime on TeraBox's side, and
// polling it longer can never receive a phone confirmation.
func qrStateExpired(state *qrLoginState) bool {
	if state.CreatedAt == 0 {
		return true // legacy state persisted before timestamps existed
	}
	return time.Since(time.Unix(state.CreatedAt, 0)) > qrStateMaxAge
}

func originOf(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func cookieFromJar(jar *cookiejar.Jar, u *url.URL, name string) string {
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
