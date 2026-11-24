package terabox

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestQRLoginE2EProbe runs the real QR login flow against TeraBox to prove the
// poll request is accepted by the /passport/qrcode/login endpoint.
//
// It never completes a login (nobody scans the QR); it asserts that a poll
// made by pollQRStatus comes back as pending (code 39 "login timeout" after
// the long-poll window, or code 0 without ndus) instead of the hard rejection
// {"code":6,"msg":"System error, please try again later"} seen before the fix.
//
// Only runs when TERABOX_QR_E2E=1 (real network access, ~30-40s long-poll).
func TestQRLoginE2EProbe(t *testing.T) {
	if os.Getenv("TERABOX_QR_E2E") != "1" {
		t.Skip("set TERABOX_QR_E2E=1 to run the real-network probe")
	}

	d := &Terabox{}
	state, _, err := d.startQRLogin(context.Background())
	if err != nil {
		t.Fatalf("startQRLogin() error = %v", err)
	}
	t.Logf("QR obtained: uuid=%s seq=%d browserid=%s page=%s",
		state.UUID, state.Seq, state.Browserid, state.PageURL)

	start := time.Now()
	_, _, err = d.pollQRStatus(context.Background(), state)
	if err != errQRPending {
		t.Fatalf("pollQRStatus() err = %v\n"+
			"want errQRPending: the endpoint must accept the poll (long-poll -> 39 'login timeout');\n"+
			"'QR login failed: {\"code\":6...}' means the request shape is still being rejected", err)
	}
	t.Logf("poll accepted after %v (long-poll window) — endpoint answered pending, not code 6",
		time.Since(start).Round(time.Millisecond))
}
