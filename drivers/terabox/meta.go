package terabox

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootPath
	Cookie string `json:"cookie"` // optional: filled by QR login when empty; manual cookie paste still supported as fallback
	// QR login state (frontend re-sends after scanning; server polls on next save).
	// Hidden from the driver form: it is internal state, not a user input.
	QRState *qrLoginState `json:"qrcode_state,omitempty" ignore:"true"`
	//JsToken        string `json:"js_token" type:"string" required:"true"`
	DownloadAPI    string `json:"download_api" type:"select" options:"official,crack" default:"official"`
	OrderBy        string `json:"order_by" type:"select" options:"name,time,size" default:"name"`
	OrderDirection string `json:"order_direction" type:"select" options:"asc,desc" default:"asc"`
}

var config = driver.Config{
	Name:        "Terabox",
	DefaultRoot: "/",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Terabox{}
	})
}
