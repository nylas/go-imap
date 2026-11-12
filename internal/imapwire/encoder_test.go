package imapwire_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2/internal/imapwire"
)

func TestEncoderMailbox(t *testing.T) {
	for _, tc := range []struct {
		quotedUTF8 bool
		name, want string
	}{
		// IMAP4rev2 and UTF8=ACCEPT carry mailbox names as raw UTF-8, so the
		// server must get back the exact name it sent in LIST.
		{true, "A - Vendors/T&S Property", `"A - Vendors/T&S Property"`},
		{true, "Café & Bar", `"Café & Bar"`},
		{false, "A - Vendors/T&S Property", `"A - Vendors/T&-S Property"`},
		{false, "Café & Bar", `"Caf&AOk- &- Bar"`},
	} {
		var sb strings.Builder
		bw := bufio.NewWriter(&sb)
		enc := imapwire.NewEncoder(bw, imapwire.ConnSideClient)
		enc.QuotedUTF8 = tc.quotedUTF8
		if err := enc.Mailbox(tc.name).CRLF(); err != nil {
			t.Fatalf("Mailbox(%q) = %v", tc.name, err)
		}
		if got := strings.TrimSuffix(sb.String(), "\r\n"); got != tc.want {
			t.Errorf("Mailbox(%q) with QuotedUTF8=%v = %s, want %s", tc.name, tc.quotedUTF8, got, tc.want)
		}
	}
}

func TestMailboxRoundTripQuotedUTF8(t *testing.T) {
	for _, name := range []string{"A - Customer Emails/Industrial Caulk & Seal", "R&-D"} {
		dec := newTestDecoder(`"` + name + `"` + "\r\n")
		dec.QuotedUTF8 = true
		var decoded string
		if !dec.ExpectMailbox(&decoded) {
			t.Fatalf("ExpectMailbox() = %v", dec.Err())
		}
		if decoded != name {
			t.Errorf("ExpectMailbox() = %q, want %q", decoded, name)
		}

		var sb strings.Builder
		bw := bufio.NewWriter(&sb)
		enc := imapwire.NewEncoder(bw, imapwire.ConnSideClient)
		enc.QuotedUTF8 = true
		if err := enc.Mailbox(decoded).CRLF(); err != nil {
			t.Fatalf("Mailbox(%q) = %v", decoded, err)
		}
		if got, want := sb.String(), `"`+name+`"`+"\r\n"; got != want {
			t.Errorf("mailbox read from server encoded as %q, want %q", got, want)
		}
	}
}
