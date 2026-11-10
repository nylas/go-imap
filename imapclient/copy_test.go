package imapclient_test

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

// Purelymail returns "*" in COPYUID UID sets, which RFC 4315 forbids
func TestCopy_dynamicCOPYUID(t *testing.T) {
	client := newScriptedClient(t, "TAG OK [COPYUID 1 1:* 5:*] COPY completed\r\n")

	data, err := client.Copy(imap.SeqSetNum(1), "Archive").Wait()
	if err != nil {
		t.Fatalf("Copy() = %v", err)
	}
	if data.UIDValidity != 0 || data.SourceUIDs != nil || data.DestUIDs != nil {
		t.Errorf("Copy() = %+v, want no COPYUID data", data)
	}
}
