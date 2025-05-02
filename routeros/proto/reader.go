//nolint:gomnd,mnd
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
			} else if len(t) > 1 {
				sen.Map[string(t[0])] = string(t[1])
			}

			continue
		}

		return nil, InvalidSentenceWordError{buf}
	}
}

func (r *reader) readBytes(buf []byte, size int) error {
	if _, err := io.ReadAtLeast(r, buf, size); err != nil {
		return fmt.Errorf("read to buffer error: %w", err)
	}

	return nil
}

func (r *reader) readLength() (int64, error) {
	buf := []byte{0, 0, 0, 0}

	_, err := r.Read(buf[3:])
	if err != nil {
		return -1, fmt.Errorf("read to buffer error: %w", err)
	}

	res := int64(buf[3])

	switch {
	case res&0x80 == 0x00:
	case (res & 0xC0) == 0x80:
		buf[2] = byte(res & ^0xC0)
		err = r.readBytes(buf[3:], 1)
	case res&0xE0 == 0xC0:
		buf[1] = byte(res & ^0xE0)
		err = r.readBytes(buf[2:], 2)
	case res&0xF0 == 0xE0:
		buf[0] = byte(res & ^0xF0)
		err = r.readBytes(buf[1:], 3)
	case res&0xF8 == 0xF0:
		err = r.readBytes(buf, 4)
	}

	if err != nil {
		return -1, err
	}

	res = 0
	for _, v := range buf {
		res = res<<8 | int64(v)
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
