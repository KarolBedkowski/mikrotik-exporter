//nolint:gomnd
package proto

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"golang.org/x/text/encoding/charmap"
)

// Reader reads sentences from a RouterOS device.
type Reader interface {
	ReadSentence() (*Sentence, error)
}

type reader struct {
	*bufio.Reader
}

// NewReader returns a new Reader to read from r.
func NewReader(r io.Reader) Reader {
	return &reader{bufio.NewReader(r)}
}

// ReadSentence reads a sentence.
func (r *reader) ReadSentence() (*Sentence, error) {
	sen := NewSentence()

	for {
		buf, err := r.readWord()
		if err != nil {
			return nil, err
		}

		if len(buf) == 0 {
			return sen, nil
		}
		// Ex.: !re, !done
		if sen.Word == "" {
			sen.Word = string(buf)

			continue
		}
		// Command tag.
		if bytes.HasPrefix(buf, []byte(".tag=")) {
			sen.Tag = string(buf[5:])

			continue
		}
		// Ex.: =key=value, =key
		if buf[0] == '=' {
			if t := bytes.SplitN(buf[1:], []byte("="), 2); len(t) == 1 {
				sen.Map[string(t[0])] = ""
			} else {
				sen.Map[string(t[0])] = string(t[1])
			}

			continue
		}

		return nil, InvalidSentenceWordError{buf}
	}
}

func (r *reader) readNumber(size int) (int64, error) {
	b := make([]byte, size)
	if _, err := io.ReadFull(r, b); err != nil {
		return -1, fmt.Errorf("read to buffer error: %w", err)
	}

	var num int64
	for _, ch := range b {
		num = num<<8 | int64(ch)
	}

	return num, nil
}

func (r *reader) readLength() (int64, error) {
	res, err := r.readNumber(1)
	if err != nil {
		return -1, err
	}

	var n int64

	switch {
	case res&0x80 == 0x00:
	case (res & 0xC0) == 0x80:
		n, err = r.readNumber(1)
		res = res & ^0xC0 << 8 | n
	case res&0xE0 == 0xC0:
		n, err = r.readNumber(2)
		res = res & ^0xE0 << 16 | n
	case res&0xF0 == 0xE0:
		n, err = r.readNumber(3)
		res = res & ^0xF0 << 24 | n
	case res&0xF8 == 0xF0:
		res, err = r.readNumber(4)
	}

	if err != nil {
		return -1, err
	}

	return res, nil
}

func (r *reader) readWord() ([]byte, error) {
	l, err := r.readLength()
	if err != nil {
		return nil, err
	}

	b := make([]byte, l)
	if _, err = io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read word to buffer error: %w", err)
	}

	d := charmap.Windows1250.NewDecoder()

	buf, err := d.Bytes(b)
	if err != nil {
		return nil, fmt.Errorf("decode result error: %w", err)
	}

	return buf, nil
}
