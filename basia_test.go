package main

import (
	"testing"

	differ "github.com/kylelemons/godebug/diff"
)

func TestWrap70(t *testing.T) {
	got := wrap70("" +
		//234567890
		".bcdefgh.1.bcdefgh.2.bcdefgh.3.bcdefgh.4.bcdefgh.5.bcdefgh.6.bcdefgh.7" +
		".bcdefgh.A.bcdefgh.B.bcdefgh.C.bcdefgh.D.bcdefgh.E.bcdefgh.F.bcdefgh.G" +
		".bcdefgh.H.bcdefgh.I.bcdefgh.J.bcdefgh.K.bcdefgh.L.bcdefgh.M.bcdefgh")
	want := "" +
		".bcdefgh.1.bcdefgh.2.bcdefgh.3.bcdefgh.4.bcdefgh.5.bcdefgh.6.bcdefgh.7\r\n" +
		" .bcdefgh.A.bcdefgh.B.bcdefgh.C.bcdefgh.D.bcdefgh.E.bcdefgh.F.bcdefgh.\r\n" +
		" G.bcdefgh.H.bcdefgh.I.bcdefgh.J.bcdefgh.K.bcdefgh.L.bcdefgh.M.bcdefgh"
	if diff := differ.Diff(got, want); diff != "" {
		t.Errorf("bad wrap, diff (-have +want):\n%s", diff)
	}
}
