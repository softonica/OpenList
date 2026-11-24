package terabox

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
)

// ensureRestyClient initializes the shared driver HTTP client outside server
// bootstrap (base.InitClient only runs in the real server).
func ensureRestyClient() {
	if conf.Conf == nil {
		conf.Conf = conf.DefaultConfig("testdata")
	}
	if base.RestyClient == nil {
		base.RestyClient = base.NewRestyClient()
	}
}

// A storage whose login never completed has no cookie; the list API answers
// {"errno":-6} ("not logged in" — verified live 2026-11-24 against
// www.terabox.com/api/list). The driver must surface that as an error instead
// of silently rendering an empty folder (observed live on mount /TBtestQR:
// the storage looked mounted but listing was empty).
func TestGetFilesErrorsOnNonZeroErrno(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":-6,"request_id":8758809509134968821}`))
	}))
	defer srv.Close()

	d := &Terabox{}
	d.base_url = srv.URL
	ensureRestyClient()

	files, err := d.getFiles("/")
	if err == nil || !strings.Contains(err.Error(), "-6") {
		t.Fatalf("getFiles err = %v, want error carrying errno -6", err)
	}
	if files != nil {
		t.Errorf("files = %v, want nil on error", files)
	}
}

// errno 0 with an empty list is a genuinely empty directory: no error.
func TestGetFilesEmptyDirIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":0,"list":[]}`))
	}))
	defer srv.Close()

	d := &Terabox{}
	d.base_url = srv.URL
	ensureRestyClient()

	files, err := d.getFiles("/")
	if err != nil {
		t.Fatalf("getFiles err = %v, want nil for a genuinely empty dir", err)
	}
	if len(files) != 0 {
		t.Errorf("files = %v, want empty", files)
	}
}

// errno 9000 keeps its dedicated area-unavailable message.
func TestGetFilesAreaUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":9000}`))
	}))
	defer srv.Close()

	d := &Terabox{}
	d.base_url = srv.URL
	ensureRestyClient()

	_, err := d.getFiles("/")
	if err == nil || !strings.Contains(err.Error(), "not yet available") {
		t.Fatalf("getFiles err = %v, want area-unavailable error", err)
	}
}
