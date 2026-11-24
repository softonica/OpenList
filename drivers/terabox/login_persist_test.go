package terabox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var (
	setupDBOnce sync.Once
	setupDBErr  error
	rowSeq      int
)

// setupTestDB mirrors drivers/115_open/driver_test.go: an in-memory SQLite
// database behind internal/db, so op.MustSaveDriverStorage (called by
// op.initStorage after Init) really persists and the test can assert on the
// stored row.
func setupTestDB(t *testing.T) {
	t.Helper()
	setupDBOnce.Do(func() {
		var gormDB *gorm.DB
		gormDB, setupDBErr = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if setupDBErr != nil {
			return
		}
		conf.Conf = conf.DefaultConfig("testdata")
		db.Init(gormDB)
	})
	if setupDBErr != nil {
		t.Fatalf("failed to set up test database: %v", setupDBErr)
	}
}

func teraboxPassportMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ai/index", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "browserid", Value: "BROWSER_1", Path: "/"})
		_, _ = w.Write([]byte(`<html><script>"pcftoken":"PCFT_1"</script></body></html>`))
	})
	mux.HandleFunc("/passport/qrcode/get", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"code":0,"data":{"uuid":"UUID_1","qrcode":"data:image/png;base64,iVBOR","seq":7}}`)
	})
	mux.HandleFunc("/passport/qrcode/login", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"code":0,"data":{"ndus":"NDUS_PERSISTED","uname":"alice"}}`)
	})
	return mux
}

func decodeAddition(t *testing.T, raw string) Addition {
	t.Helper()
	var add Addition
	if err := json.Unmarshal([]byte(raw), &add); err != nil {
		t.Fatalf("failed to decode persisted addition %q: %v", raw, err)
	}
	return add
}

// The design contract of the QR login feature: QR is only an *alternative* way
// to obtain the cookie. Once the login succeeds during a save, the ndus cookie
// must be persisted permanently to the database (and the one-shot QR state
// cleared), so a restart keeps working from the stored cookie alone.
//
// This walks the real op.CreateStorage / op.UpdateStorage flow against mock
// TeraBox endpoints, exactly like the frontend does: create (need verify, QR
// state persisted) → save again with the round-tripped state (poll → ndus).
func TestQrLoginPersistsCookieViaOpFlow(t *testing.T) {
	setupTestDB(t)
	srv := httptest.NewServer(teraboxPassportMux(t))
	defer srv.Close()
	setQRLoginPageURL(t, srv.URL+"/ai/index")

	rowSeq++
	mountPath := fmt.Sprintf("/terabox-persist-test-%d", rowSeq)
	ctx := context.Background()

	// 1) Create the storage with an empty cookie → "need verify" error carries
	// the QR page; the framework persists the QR state for the next save.
	id, err := op.CreateStorage(ctx, model.Storage{
		Driver:    "Terabox",
		MountPath: mountPath,
		Addition:  `{"cookie":""}`,
	})
	if err == nil || !strings.Contains(err.Error(), "need verify") {
		t.Fatalf("CreateStorage err = %v, want need verify (QR page)", err)
	}

	row, err := db.GetStorageById(id)
	if err != nil {
		t.Fatalf("failed to read storage row after create: %v", err)
	}
	add := decodeAddition(t, row.Addition)
	if add.Cookie != "" {
		t.Errorf("after create, cookie = %q, want empty", add.Cookie)
	}
	if add.QRState == nil || add.QRState.UUID != "UUID_1" {
		t.Fatalf("after create, QRState = %+v, want state with UUID_1 (persisted for the next save)", add.QRState)
	}

	// 2) The frontend round-trip: save again, re-sending the addition holding
	// the QR state. The user confirmed in the app, so the poll succeeds.
	err = op.UpdateStorage(ctx, model.Storage{
		ID:        id,
		Driver:    "Terabox",
		MountPath: mountPath,
		Addition:  row.Addition,
	})
	if err != nil {
		t.Fatalf("UpdateStorage err = %v, want QR login to succeed", err)
	}

	// 3) THE CONTRACT: the ndus cookie is persisted permanently and the
	// one-shot QR state is gone, so restarts load from the cookie alone.
	row2, err := db.GetStorageById(id)
	if err != nil {
		t.Fatalf("failed to read storage row after update: %v", err)
	}
	add2 := decodeAddition(t, row2.Addition)
	if add2.Cookie != "ndus=NDUS_PERSISTED" {
		t.Errorf("persisted cookie = %q, want ndus=NDUS_PERSISTED (permanent login)", add2.Cookie)
	}
	if add2.QRState != nil {
		t.Errorf("persisted QRState = %+v, want nil (one-shot state cleared)", add2.QRState)
	}
}
