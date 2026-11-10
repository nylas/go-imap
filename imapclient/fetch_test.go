package imapclient_test

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func TestFetch(t *testing.T) {
	client, server := newClientServerPair(t, imap.ConnStateSelected)
	defer client.Close()
	defer server.Close()

	seqSet := imap.SeqSetNum(1)
	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}
	messages, err := client.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		t.Fatalf("failed to fetch first message: %v", err)
	} else if len(messages) != 1 {
		t.Fatalf("len(messages) = %v, want 1", len(messages))
	}

	msg := messages[0]
	if len(msg.BodySection) != 1 {
		t.Fatalf("len(msg.BodySection) = %v, want 1", len(msg.BodySection))
	}
	b := msg.FindBodySection(bodySection)
	if b == nil {
		t.Fatalf("FindBodySection() = nil")
	}
	body := strings.ReplaceAll(string(b), "\r\n", "\n")
	if body != simpleRawMessage {
		t.Errorf("body mismatch: got \n%v\n but want \n%v", body, simpleRawMessage)
	}
}

func TestFetch_envelopeNonCompliant(t *testing.T) {
	// Some servers send NIL for the envelope, NIL inside an address list, or a
	// SP before the envelope's ")"
	for _, envelope := range []string{
		`NIL`,
		`(NIL "hi" (NIL ("Bob" NIL "bob" "example.org")) NIL NIL NIL NIL NIL NIL NIL)`,
		`(NIL "hi" (("Bob" NIL "bob" "example.org")) NIL NIL NIL NIL NIL NIL NIL )`,
	} {
		clientConn, serverConn := net.Pipe()
		client := imapclient.New(clientConn, nil)

		go func() {
			serverConn.Write([]byte("* OK ready\r\n"))
			if _, err := bufio.NewReader(serverConn).ReadString('\n'); err != nil {
				return
			}
			serverConn.Write([]byte("* 1 FETCH (UID 1 ENVELOPE " + envelope + ")\r\nT1 OK done\r\n"))
		}()

		msgs, err := client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{Envelope: true}).Collect()
		if err != nil {
			t.Errorf("Fetch(ENVELOPE %v) = %v", envelope, err)
		} else if len(msgs) != 1 || msgs[0].UID != 1 {
			t.Errorf("Fetch(ENVELOPE %v) = %v, want UID 1", envelope, msgs)
		} else if env := msgs[0].Envelope; (env == nil) != (envelope == "NIL") || env != nil && (env.Subject != "hi" || len(env.From) != 1 || env.From[0].Mailbox != "bob") {
			t.Errorf("Fetch(ENVELOPE %v) envelope = %+v", envelope, env)
		}

		client.Close()
		serverConn.Close()
	}
}

func TestFetch_emptyLiteralMissingSP(t *testing.T) {
	// Open-Xchange omits the SP after an empty literal. Extra SPs must still be
	// accepted too.
	for _, resp := range []string{
		"* 1 FETCH (BODY[HEADER.FIELDS (REFERENCES)] {0}\r\nUID 42)\r\n",
		"* 1 FETCH ( BODY[HEADER.FIELDS (REFERENCES)] {0}\r\n  UID 42 )\r\n",
	} {
		client := newScriptedClient(t, resp+"TAG OK FETCH completed\r\n")
		msgs, err := client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
			UID:         true,
			BodySection: []*imap.FetchItemBodySection{{Specifier: imap.PartSpecifierHeader, HeaderFields: []string{"References"}, Peek: true}},
		}).Collect()
		if err != nil {
			t.Errorf("Fetch(%q) = %v", resp, err)
		} else if len(msgs) != 1 || msgs[0].UID != 42 {
			t.Errorf("Fetch(%q) = %v, want UID 42", resp, msgs)
		}
	}
}

func TestFetch_missingSPAfterNonEmptyLiteral(t *testing.T) {
	client := newScriptedClient(t, "* 1 FETCH (BODY[] {2}\r\nhiUID 42)\r\nTAG OK FETCH completed\r\n")
	_, err := client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{{}},
	}).Collect()
	if err == nil {
		t.Errorf("Fetch() = nil, want an error for a missing SP after a non-empty literal")
	}
}

func TestFetch_bodyStructureNonCompliant(t *testing.T) {
	for _, tc := range []struct {
		name, bs string
		check    func(imap.BodyStructure) bool
	}{
		{
			// QQ Mail
			name: "NIL type",
			bs:   `(NIL NIL ("CHARSET" "GB2312") NIL NIL NIL 7 NIL NIL NIL)`,
			check: func(bs imap.BodyStructure) bool {
				sp, ok := bs.(*imap.BodyStructureSinglePart)
				return ok && sp.Type == "" && sp.Subtype == "" && sp.Size == 7 && sp.Params["charset"] == "GB2312"
			},
		},
		{
			name: "NIL subtype",
			bs:   `("TEXT" NIL ("CHARSET" "US-ASCII") NIL NIL "7BIT" 12 1 NIL NIL NIL)`,
			check: func(bs imap.BodyStructure) bool {
				sp, ok := bs.(*imap.BodyStructureSinglePart)
				return ok && sp.Type == "TEXT" && sp.Subtype == "" && sp.Text != nil && sp.Text.NumLines == 1
			},
		},
		{
			// Dovecot omits the envelope, body and lines for message/global
			name: "message/global without envelope",
			bs:   `(("TEXT" "PLAIN" ("CHARSET" "UTF-8") NIL NIL "7BIT" 5 1 NIL NIL NIL NIL)("MESSAGE" "GLOBAL" NIL NIL NIL "7BIT" 100 NIL ("attachment" NIL) NIL NIL) "MIXED" ("BOUNDARY" "b") NIL NIL NIL)`,
			check: func(bs imap.BodyStructure) bool {
				mp, ok := bs.(*imap.BodyStructureMultiPart)
				if !ok || len(mp.Children) != 2 {
					return false
				}
				sp, ok := mp.Children[1].(*imap.BodyStructureSinglePart)
				return ok && strings.EqualFold(sp.Subtype, "global") && sp.MessageRFC822 == nil && sp.Size == 100 && sp.Extended != nil && sp.Extended.Disposition != nil
			},
		},
	} {
		client := newScriptedClient(t, "* 1 FETCH (UID 1 BODYSTRUCTURE "+tc.bs+")\r\nTAG OK FETCH completed\r\n")
		msgs, err := client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
			UID:           true,
			BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		}).Collect()
		if err != nil {
			t.Errorf("%v: Fetch() = %v", tc.name, err)
		} else if len(msgs) != 1 || !tc.check(msgs[0].BodyStructure) {
			t.Errorf("%v: BodyStructure = %#v", tc.name, msgs)
		}
	}
}
