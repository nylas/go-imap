package imapclient_test

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

func TestSelect(t *testing.T) {
	client, server := newClientServerPair(t, imap.ConnStateAuthenticated)
	defer client.Close()
	defer server.Close()

	data, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		t.Fatalf("Select() = %v", err)
	} else if data.NumMessages != 1 {
		t.Errorf("SelectData.NumMessages = %v, want %v", data.NumMessages, 1)
	}
}

// imap.postoffice.net sends resp-text-codes with an empty number
func TestSelect_emptyUIDNextUIDValidity(t *testing.T) {
	client := newScriptedClient(t, "* 3 EXISTS\r\n"+
		"* OK [UIDVALIDITY ] UIDs valid\r\n"+
		"* OK [UIDNEXT ] Predicted next UID\r\n"+
		"TAG OK [READ-WRITE] SELECT completed\r\n")

	data, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		t.Fatalf("Select() = %v", err)
	}
	if data.NumMessages != 3 || data.UIDValidity != 0 || data.UIDNext != 0 {
		t.Errorf("Select() = %+v, want NumMessages 3 and zero UIDValidity/UIDNext", data)
	}
}

// Gmail accepts keywords containing "]" and lists them in FLAGS and
// PERMANENTFLAGS, so rejecting them fails every SELECT
func TestSelect_keywordWithBracket(t *testing.T) {
	client := newScriptedClient(t, "* FLAGS (\\Seen [Imap]/Todo)\r\n"+
		"* OK [PERMANENTFLAGS (\\Seen [Imap]/Todo \\*)] Flags permitted.\r\n"+
		"* 1 EXISTS\r\n"+
		"TAG OK [READ-WRITE] SELECT completed\r\n")

	data, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		t.Fatalf("Select() = %v", err)
	}
	if !containsFlag(data.Flags, "[Imap]/Todo") {
		t.Errorf("SelectData.Flags = %v, want it to contain [Imap]/Todo", data.Flags)
	}
	if !containsFlag(data.PermanentFlags, "[Imap]/Todo") || !containsFlag(data.PermanentFlags, imap.FlagWildcard) {
		t.Errorf("SelectData.PermanentFlags = %v, want [Imap]/Todo and \\*", data.PermanentFlags)
	}
}

func containsFlag(flags []imap.Flag, flag imap.Flag) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}
