package imapwire_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2/internal/imapwire"
)

func newTestDecoder(s string) *imapwire.Decoder {
	return imapwire.NewDecoder(bufio.NewReader(strings.NewReader(s)), imapwire.ConnSideServer)
}

func TestExpectListExtraSP(t *testing.T) {
	// Some servers send extra SPs in lists, e.g. "FLAGS (\Seen )" or "LIST ( \HasNoChildren)"
	for _, s := range []string{"( a b)", "(a b )", "(a  b)", "(  a  b  )"} {
		dec := newTestDecoder(s + "\r\n")

		var items []string
		err := dec.ExpectList(func() error {
			var item string
			if !dec.ExpectAtom(&item) {
				return dec.Err()
			}
			items = append(items, item)
			return nil
		})
		if err != nil {
			t.Errorf("ExpectList(%q) = %v", s, err)
		} else if strings.Join(items, ",") != "a,b" || !dec.ExpectCRLF() {
			t.Errorf("ExpectList(%q) = %q, want [a b] then CRLF", s, items)
		}
	}
}

func TestDiscardValueNestedList(t *testing.T) {
	for _, s := range []string{"((a))", "(a (b))", "( (a) )"} {
		dec := newTestDecoder(s + "\r\n")
		if !dec.DiscardValue() || !dec.ExpectCRLF() {
			t.Errorf("DiscardValue(%q) = %v", s, dec.Err())
		}
	}
}
