package bigone

import (
	. "github.com/Snooowgh/GoEx"
	"net/http"
	"testing"
)

var (
	bo = New(http.DefaultClient, "", "")
)

func TestBigone_GetTicker(t *testing.T) {
	t.Log(bo.GetTicker(ETH_BTC))
}
