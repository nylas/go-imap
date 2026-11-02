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

func TestFetchPartialOffsetEcho(t *testing.T) {
	client, server := newClientServerPair(t, imap.ConnStateSelected)
	defer client.Close()
	defer server.Close()

	const offset = int64(1)<<32 + 5

	bodySection := &imap.FetchItemBodySection{
		Partial: &imap.SectionPartial{Offset: offset, Size: 10},
	}
	messages, err := client.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}).Collect()
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(messages) != 1 || len(messages[0].BodySection) != 1 {
		t.Fatalf("got %v messages", len(messages))
	}

	got := messages[0].BodySection[0].Section.Partial
	if got == nil || got.Offset != offset {
		t.Errorf("echoed partial offset = %v, want %v", got, offset)
	}
	if messages[0].FindBodySection(bodySection) == nil {
		t.Errorf("FindBodySection() = nil, want the section the client asked for")
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
