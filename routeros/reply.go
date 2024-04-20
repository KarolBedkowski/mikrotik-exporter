package routeros

import (
	"bytes"
	"fmt"

	"mikrotik-exporter/routeros/proto"
)

// Reply has all the sentences from a reply.
type Reply struct {
	Done *proto.Sentence
	Re   []*proto.Sentence
}

func (r *Reply) String() string {
	b := &bytes.Buffer{}
	for _, re := range r.Re {
		fmt.Fprintf(b, "%s\n", re)
	}

	fmt.Fprintf(b, "%s", r.Done)

	return b.String()
}

// readReply reads one reply synchronously. It returns the reply.
func (c *Client) readReply() (*Reply, error) {
	reply := &Reply{}

	var lastErr error

	for {
		sen, err := c.r.ReadSentence()
		if err != nil {
			return nil, fmt.Errorf("read sentence error: %w", err)
		}

		done, err := reply.processSentence(sen)
		if err != nil {
			if done {
				return nil, err
			}

			lastErr = err
		}

		if done {
			return reply, lastErr
		}
	}
}

func (r *Reply) processSentence(sen *proto.Sentence) (bool, error) {
	switch sen.Word {
	case "!re":
		r.Re = append(r.Re, sen)
	case "!done":
		r.Done = sen

		return true, nil
	case "!trap", "!fatal":
		return sen.Word == "!fatal", &DeviceError{sen}
	case "":
		// API docs say that empty sentences should be ignored
	default:
		return true, &UnknownReplyError{sen}
	}

	return false, nil
}
