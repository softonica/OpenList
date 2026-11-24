package terabox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func setQRLoginPageURL(t *testing.T, pageURL string) {
	t.Helper()
	old := qrLoginPageURL
	qrLoginPageURL = pageURL
	t.Cleanup(func() { qrLoginPageURL = old })
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func qrPageMux(t *testing.T, qrGetBody string) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
		_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
	})
	mux.HandleFunc("/passport/qrcode/get", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, qrGetBody)
	})
	return mux
}

func TestQrLoginStateForm(t *testing.T) {
	s := &qrLoginState{
		Browserid: "B1", Client: "web", Clientfrom: "h5", Lang: "en",
		PassVersion: "2.8", PCFToken: "P1", RegSource: "web", Seq: 7, UUID: "U1",
	}
	want := map[string]string{
		"browserid": "B1", "client": "web", "clientfrom": "h5", "lang": "en",
		"pass_version": "2.8", "pcftoken": "P1", "reg_source": "web", "seq": "7", "uuid": "U1",
	}
	got := s.form()
	if len(got) != len(want) {
		t.Fatalf("form() = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("form()[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestOriginOf(t *testing.T) {
	if got := originOf("https://www.1024terabox.com/ai/index"); got != "https://www.1024terabox.com" {
		t.Errorf("originOf() = %q", got)
	}
	if got := originOf(""); got != "" {
		t.Errorf("originOf(\"\") = %q, want \"\"", got)
	}
}

func TestStartQRLoginSuccess(t *testing.T) {
	srv := httptest.NewServer(qrPageMux(t, `{"code":0,"data":{"uuid":"UUID_1","qrcode":"data:image/png;base64,iVBORw0KGgo=","seq":123}}`))
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	state, qrPage, err := d.startQRLogin(context.Background())
	if err != nil {
		t.Fatalf("startQRLogin() error = %v", err)
	}
	if state.Browserid != "BROWSER_1" {
		t.Errorf("state.Browserid = %q, want BROWSER_1", state.Browserid)
	}
	if state.PCFToken != "PCFT_1" {
		t.Errorf("state.PCFToken = %q, want PCFT_1", state.PCFToken)
	}
	if state.UUID != "UUID_1" || state.Seq != 123 {
		t.Errorf("state.UUID/Seq = %q/%d, want UUID_1/123", state.UUID, state.Seq)
	}
	if state.Client != "web" || state.Clientfrom != "h5" || state.PassVersion != "2.8" || state.RegSource != "web" || state.Lang != "en" {
		t.Errorf("state fields wrong: %+v", state)
	}
	if !strings.Contains(state.PageURL, "/ai/index") {
		t.Errorf("state.PageURL = %q", state.PageURL)
	}
	if !strings.Contains(qrPage, "data:image/png;base64,iVBORw0KGgo=") {
		t.Errorf("qrPage missing QR data URI: %q", qrPage)
	}
}

func TestStartQRLoginNoBrowserid(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>"pcftoken":"PCFT_1"</html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	_, _, err := d.startQRLogin(context.Background())
	if err == nil || !strings.Contains(err.Error(), "browserid") {
		t.Fatalf("err = %v, want browserid error", err)
	}
}

func TestStartQRLoginNoPCFToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "B1", Path: "/"})
		_, _ = w.Write([]byte(`<html>no token here</html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	_, _, err := d.startQRLogin(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pcftoken") {
		t.Fatalf("err = %v, want pcftoken error", err)
	}
}

func TestStartQRLoginFail(t *testing.T) {
	srv := httptest.NewServer(qrPageMux(t, `{"code":104}`))
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	_, _, err := d.startQRLogin(context.Background())
	if err == nil || !strings.Contains(err.Error(), "QR request failed") {
		t.Fatalf("err = %v, want QR request failed error", err)
	}
}

func TestPollQRStatusSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"code":0,"data":{"ndus":"NDUS_VAL","uname":"alice"}}`)
	}))
	defer srv.Close()

	d := &Terabox{}
	state := &qrLoginState{PageURL: srv.URL + "/ai/index", UUID: "U", PCFToken: "P", Seq: 1}
	ndus, uname, err := d.pollQRStatus(context.Background(), state)
	if err != nil {
		t.Fatalf("pollQRStatus() error = %v", err)
	}
	if ndus != "NDUS_VAL" {
		t.Errorf("ndus = %q, want NDUS_VAL", ndus)
	}
	if uname != "alice" {
		t.Errorf("uname = %q, want alice", uname)
	}
}

func TestPollQRStatusNDusFromJar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "ndus", Value: "JAR_NDUS", Path: "/"})
		writeJSON(w, `{"code":0}`)
	}))
	defer srv.Close()

	d := &Terabox{}
	state := &qrLoginState{PageURL: srv.URL + "/ai/index", UUID: "U", PCFToken: "P", Seq: 1}
	ndus, _, err := d.pollQRStatus(context.Background(), state)
	if err != nil {
		t.Fatalf("pollQRStatus() error = %v", err)
	}
	if ndus != "JAR_NDUS" {
		t.Errorf("ndus = %q, want JAR_NDUS from jar", ndus)
	}
}

func TestPollQRStatusPending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"code":39}`)
	}))
	defer srv.Close()

	d := &Terabox{}
	state := &qrLoginState{PageURL: srv.URL + "/ai/index", UUID: "U", PCFToken: "P", Seq: 1}
	_, _, err := d.pollQRStatus(context.Background(), state)
	if err != errQRPending {
		t.Fatalf("err = %v, want errQRPending", err)
	}
}

func TestPollQRStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"code":104}`)
	}))
	defer srv.Close()

	d := &Terabox{}
	state := &qrLoginState{PageURL: srv.URL + "/ai/index", UUID: "U", PCFToken: "P", Seq: 1}
	_, _, err := d.pollQRStatus(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "QR login failed") {
		t.Fatalf("err = %v, want QR login failed error", err)
	}
}

// The /passport/qrcode/login endpoint rejects cookieless, headerless requests
// with code 6 "System error" (verified against the real endpoint). The poll
// must carry the browserid cookie and browser-like headers, like the
// mill-master/rclone-terabox reference implementation does.
func TestPollQRStatusSendsBrowserContext(t *testing.T) {
	var cookie, ua, origin, referer, xhr, accept, formBrowserid string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		cookie = r.Header.Get("Cookie")
		ua = r.Header.Get("User-Agent")
		origin = r.Header.Get("Origin")
		referer = r.Header.Get("Referer")
		xhr = r.Header.Get("X-Requested-With")
		accept = r.Header.Get("Accept")
		formBrowserid = r.PostFormValue("browserid")
		writeJSON(w, `{"code":39}`)
	}))
	defer srv.Close()

	state := &qrLoginState{PageURL: srv.URL + "/ai/index", Browserid: "BRZ_1", UUID: "U", PCFToken: "P", Seq: 1}
	_, _, err := (&Terabox{}).pollQRStatus(context.Background(), state)
	if err != errQRPending {
		t.Fatalf("err = %v, want errQRPending", err)
	}

	if !strings.Contains(cookie, "browserid=BRZ_1") {
		t.Errorf("poll Cookie = %q, want browserid=BRZ_1", cookie)
	}
	if ua != qrLoginUserAgent {
		t.Errorf("poll User-Agent = %q, want %q", ua, qrLoginUserAgent)
	}
	if origin != srv.URL {
		t.Errorf("poll Origin = %q, want %q", origin, srv.URL)
	}
	if referer != state.PageURL {
		t.Errorf("poll Referer = %q, want %q", referer, state.PageURL)
	}
	if xhr != "XMLHttpRequest" {
		t.Errorf("poll X-Requested-With = %q, want XMLHttpRequest", xhr)
	}
	if !strings.Contains(accept, "application/json") {
		t.Errorf("poll Accept = %q, want application/json", accept)
	}
	if formBrowserid != "BRZ_1" {
		t.Errorf("poll form browserid = %q, want BRZ_1", formBrowserid)
	}
}

// A scanned account may need extra verification: the poll reply carries v/vcode
// challenge tokens that must be echoed on the next poll (reference behavior).
func TestPollQRStatusEchoesChallengeTokens(t *testing.T) {
	var calls int
	var lastForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = r.ParseForm()
		lastForm = r.PostForm
		if calls == 1 {
			writeJSON(w, `{"code":0,"vcode":"VC_1","data":{}}`)
			return
		}
		writeJSON(w, `{"code":0,"data":{"ndus":"NDUS_OK","uname":"bob"}}`)
	}))
	defer srv.Close()

	state := &qrLoginState{PageURL: srv.URL + "/ai/index", Browserid: "B", UUID: "U", PCFToken: "P", Seq: 1}
	d := &Terabox{}

	if _, _, err := d.pollQRStatus(context.Background(), state); err != errQRPending {
		t.Fatalf("first poll err = %v, want errQRPending (scanned, challenge issued, no ndus yet)", err)
	}
	if state.VCode != "VC_1" {
		t.Errorf("state.VCode = %q, want VC_1 (captured from reply)", state.VCode)
	}

	ndus, _, err := d.pollQRStatus(context.Background(), state)
	if err != nil {
		t.Fatalf("second poll err = %v", err)
	}
	if ndus != "NDUS_OK" {
		t.Errorf("ndus = %q, want NDUS_OK", ndus)
	}
	if got := lastForm.Get("vcode"); got != "VC_1" {
		t.Errorf("second poll form vcode = %q, want VC_1 (echoed)", got)
	}
}

// Challenge tokens may arrive inside data instead of at the top level.
func TestPollQRStatusChallengeFromData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"code":0,"data":{"v":"V_DATA"}}`)
	}))
	defer srv.Close()

	state := &qrLoginState{PageURL: srv.URL + "/ai/index", Browserid: "B", UUID: "U", PCFToken: "P", Seq: 1}
	if _, _, err := (&Terabox{}).pollQRStatus(context.Background(), state); err != errQRPending {
		t.Fatalf("err = %v, want errQRPending", err)
	}
	if state.V != "V_DATA" {
		t.Errorf("state.V = %q, want V_DATA (captured from data)", state.V)
	}
}

// A dead QR session (uuid invalid/expired) must start a fresh QR flow instead
// of failing hard, otherwise the storage is bricked: every save would poll the
// same dead uuid forever. TeraBox signals this with codes 36 ("invalid uuid")
// and 37 ("uuid expire") — observed live on 2026-11-24.
func TestLoginByQRCodeAtSessionDeadStartsFreshQR(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
	}{
		{"invalid uuid", 36},
		{"uuid expire", 37},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var getCalls int
			mux := http.NewServeMux()
			mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
				http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
				_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
			})
			mux.HandleFunc("/passport/qrcode/get", func(w http.ResponseWriter, r *http.Request) {
				getCalls++
				writeJSON(w, `{"code":0,"data":{"uuid":"UUID_FRESH","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
			})
			mux.HandleFunc("/passport/qrcode/login", func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, `{"code":`+strconv.Itoa(tc.code)+`,"logid":1,"msg":"uuid expire"}`)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			setQRLoginPageURL(t, srv.URL+"/ai/index")

			d := &Terabox{}
			d.QRState = &qrLoginState{PageURL: srv.URL + "/ai/index", Browserid: "B", UUID: "UUID_STALE", PCFToken: "P", Seq: 1, CreatedAt: time.Now().Unix()}
			err := d.loginByQRCodeAt(context.Background(), qrLoginTimeout, qrPollInterval)

			if err == nil || !strings.Contains(err.Error(), "need verify") {
				t.Fatalf("err = %v, want need verify error (fresh QR page)", err)
			}
			if d.QRState == nil || d.QRState.UUID != "UUID_FRESH" {
				t.Fatalf("d.QRState = %+v, want fresh state with UUID_FRESH", d.QRState)
			}
			if getCalls != 1 {
				t.Errorf("qrcode/get calls = %d, want 1 (one fresh QR)", getCalls)
			}
		})
	}
}

// A stored QR older than the code's scannable lifetime must be reissued BEFORE
// polling: the TeraBox app rejects old codes as expired, so polling a stale
// session can never receive a confirmation (observed live 2026-11-24: app said
// "QR code expired" while OpenList kept polling the stored state).
func TestLoginByQRCodeAtStaleStateRefreshesQR(t *testing.T) {
	var getCalls, loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
		_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
	})
	mux.HandleFunc("/passport/qrcode/get", func(w http.ResponseWriter, r *http.Request) {
		getCalls++
		writeJSON(w, `{"code":0,"data":{"uuid":"UUID_FRESH","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
	})
	mux.HandleFunc("/passport/qrcode/login", func(w http.ResponseWriter, r *http.Request) {
		loginCalls++
		writeJSON(w, `{"code":39}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	d.QRState = &qrLoginState{
		PageURL:   srv.URL + "/ai/index",
		Browserid: "B",
		UUID:      "UUID_STALE",
		PCFToken:  "P",
		Seq:       1,
		QRImage:   "data:image/png;base64,iVBOR",
		CreatedAt: time.Now().Add(-10 * time.Minute).Unix(), // older than scannable
	}

	start := time.Now()
	err := d.loginByQRCodeAt(context.Background(), qrLoginTimeout, qrPollInterval)
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "need verify") {
		t.Fatalf("err = %v, want need verify with a fresh QR page", err)
	}
	if getCalls != 1 {
		t.Errorf("qrcode/get calls = %d, want 1 (stale QR reissued)", getCalls)
	}
	if d.QRState == nil || d.QRState.UUID != "UUID_FRESH" {
		t.Fatalf("d.QRState = %+v, want fresh state with UUID_FRESH", d.QRState)
	}
	if loginCalls != 0 {
		t.Errorf("login poll calls = %d, want 0 (no polling of an expired QR)", loginCalls)
	}
	if elapsed > 3*time.Second {
		t.Errorf("elapsed = %v, want a fast answer with the fresh QR", elapsed)
	}
}


// only gets the save request canceled by the HTTP path (observed live: proxies
// kill the request at ~100-120s with "context canceled"). The driver must
// answer with the QR page while the request is still alive — and it must KEEP
// the current session: TeraBox QR sessions live 20+ minutes, and replacing
// them here made users confirm abandoned sessions (live 2026-11-24: app said
// success, the poll kept asking a newer uuid forever).
// The poll window must account for the long-poll request time: once the
// remaining window is shorter than one more long poll (~35s), polling further
// only gets the save request canceled by the HTTP path (observed live: proxies
// kill the request at ~100-120s with "context canceled"). The driver must
// answer with the QR page while the request is still alive — and it must KEEP
// the current session (a scannable QR lives minutes; a dead session is
// detected via errQRSessionDead).
func TestLoginByQRCodeAtWindowAccountsForLongPoll(t *testing.T) {
	var loginCalls, getCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
		_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
	})
	mux.HandleFunc("/passport/qrcode/get", func(w http.ResponseWriter, r *http.Request) {
		getCalls++
		writeJSON(w, `{"code":0,"data":{"uuid":"UUID_FRESH","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
	})
	mux.HandleFunc("/passport/qrcode/login", func(w http.ResponseWriter, r *http.Request) {
		loginCalls++
		writeJSON(w, `{"code":39}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	d.QRState = &qrLoginState{PageURL: srv.URL + "/ai/index", Browserid: "B", UUID: "UUID_STALE", PCFToken: "P", Seq: 1, QRImage: "data:image/png;base64,iVBOR", CreatedAt: time.Now().Unix()}

	start := time.Now()
	err := d.loginByQRCodeAt(context.Background(), 3*time.Second, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "need verify") {
		t.Fatalf("err = %v, want need verify (QR page while request still alive)", err)
	}
	if loginCalls != 1 {
		t.Errorf("login poll calls = %d, want 1 (no pointless polls once the window cannot fit another long poll)", loginCalls)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("elapsed = %v, want under 1.5s (answer well before the path kills the request)", elapsed)
	}
	// The window timeout must NOT churn the session.
	if getCalls != 0 {
		t.Errorf("qrcode/get calls = %d, want 0 (current session reused, no fresh QR)", getCalls)
	}
	if d.QRState == nil || d.QRState.UUID != "UUID_STALE" {
		t.Errorf("d.QRState = %+v, want the SAME session (UUID_STALE unchanged)", d.QRState)
	}
}

// The QR page must come from the driver's resolved base URL (www.terabox.com
// by default, or the region domain chosen via the Url-Domain-Prefix header),
// not from a hardcoded host: TeraBox serves the same page on every alias and
// picks the region dynamically.
func TestStartQRLoginUsesDriverBaseURL(t *testing.T) {
	srv := httptest.NewServer(qrPageMux(t, `{"code":0,"data":{"uuid":"UUID_1","qrcode":"data:image/png;base64,iVBORw0KGgo=","seq":123}}`))
	defer srv.Close()

	// No override: the page must be derived from the driver base URL.
	old := qrLoginPageURL
	qrLoginPageURL = ""
	t.Cleanup(func() { qrLoginPageURL = old })

	d := &Terabox{}
	d.base_url = srv.URL

	state, qrPage, err := d.startQRLogin(context.Background())
	if err != nil {
		t.Fatalf("startQRLogin() error = %v", err)
	}
	if !strings.HasPrefix(state.PageURL, srv.URL) {
		t.Fatalf("state.PageURL = %q, want it served from the driver base URL %q", state.PageURL, srv.URL)
	}
	if state.UUID != "UUID_1" {
		t.Errorf("state.UUID = %q, want UUID_1", state.UUID)
	}
	if !strings.Contains(qrPage, "data:image/png;base64,iVBORw0KGgo=") {
		t.Errorf("qrPage missing QR data URI")
	}
}

func TestLoginByQRCodeAtSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ai/index":
			http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
			_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
		case "/passport/qrcode/get":
			writeJSON(w, `{"code":0,"data":{"uuid":"UUID_1","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
		case "/passport/qrcode/login":
			writeJSON(w, `{"code":0,"data":{"ndus":"NDUS_1","uname":"alice"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	// The resolved official/region domain from Init(): the QR login must not
	// pin it to the QR page's origin (that was the hardcoded-host bug).
	d.base_url = "https://www.terabox.com"
	if err := d.loginByQRCodeAt(context.Background(), qrLoginTimeout, qrPollInterval); err != nil {
		t.Fatalf("loginByQRCodeAt() error = %v", err)
	}
	if d.Cookie != "ndus=NDUS_1" {
		t.Errorf("d.Cookie = %q, want ndus=NDUS_1", d.Cookie)
	}
	if d.base_url != "https://www.terabox.com" {
		t.Errorf("d.base_url = %q, want the resolved domain unchanged (dynamic endpoint)", d.base_url)
	}
}

func TestLoginByQRCodeAtTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ai/index":
			http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
			_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
		case "/passport/qrcode/get":
			writeJSON(w, `{"code":0,"data":{"uuid":"UUID_1","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
		case "/passport/qrcode/login":
			writeJSON(w, `{"code":39}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	err := d.loginByQRCodeAt(context.Background(), 30*time.Millisecond, 5*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "need verify") {
		t.Fatalf("err = %v, want need verify error", err)
	}
}

func TestLoginByQRCodeAtPollError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ai/index":
			http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
			_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
		case "/passport/qrcode/get":
			writeJSON(w, `{"code":0,"data":{"uuid":"UUID_1","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
		case "/passport/qrcode/login":
			writeJSON(w, `{"code":104}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	d := &Terabox{}
	err := d.loginByQRCodeAt(context.Background(), qrLoginTimeout, qrPollInterval)
	if err == nil || !strings.Contains(err.Error(), "QR login failed") {
		t.Fatalf("err = %v, want QR login failed error", err)
	}
}
