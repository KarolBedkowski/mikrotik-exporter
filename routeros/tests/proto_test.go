package tests

import (
	"io"
	"testing"

	routeros "mikrotik-exporter/routeros"
	"mikrotik-exporter/routeros/proto"

	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/login @ [{`name` `userTest`} {`password` `passTest`}]")
		s.writeSentence(t, "!done", "=ret=abc123")
		s.readSentence(t, "/login @ [{`name` `userTest`} {`response` `0021277bff9ac7caf06aa608e46616d47f`}]")
		s.writeSentence(t, "!done")
	}()

	err := c.Login("userTest", "passTest")
	require.NoError(t, err)
}

func TestLoginIncorrect(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/login @ [{`name` `userTest`} {`password` `passTest`}]")
		s.writeSentence(t, "!done", "=ret=abc123")
		s.readSentence(t, "/login @ [{`name` `userTest`} {`response` `0021277bff9ac7caf06aa608e46616d47f`}]")
		s.writeSentence(t, "!trap", "=message=incorrect login")
		s.writeSentence(t, "!done")
	}()

	err := c.Login("userTest", "passTest")
	require.Error(t, err, "Login succeeded; want error")

	if err.Error() != "from RouterOS device: incorrect login" {
		t.Fatal(err)
	}
}

func TestLoginNoChallenge(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/login @ [{`name` `userTest`} {`password` `passTest`}]")
		s.writeSentence(t, "!done")
	}()

	err := c.Login("userTest", "passTest")
	require.NoError(t, err)
}

func TestLoginInvalidChallenge(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/login @ [{`name` `userTest`} {`password` `passTest`}]")
		s.writeSentence(t, "!done", "=ret=Invalid Hex String")
	}()

	err := c.Login("userTest", "passTest")
	require.Error(t, err, "Login succeeded; want error")
	require.ErrorContains(t, err, "RouterOS: /login: invalid ret (challenge) hex string received: encoding/hex: invalid byte: U+0049 'I'")
}

func TestLoginEOF(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()
	s.Close()

	err := c.Login("userTest", "passTest")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorContains(t, err, "endsentence error: io: read/write on closed pipe")
}

func TestCloseTwice(t *testing.T) {
	c, s := newPair(t)
	defer s.Close()
	c.Close()
	c.Close()
}

func TestRun(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
		s.writeSentence(t, "!re", "=address=1.2.3.4/32")
		s.writeSentence(t, "!done")
	}()

	sen, err := c.Run("/ip/address")
	require.NoError(t, err)
	require.Equal(t, "!re @ [{`address` `1.2.3.4/32`}]\n!done @ []", sen.String(), "/ip/address")
}

func TestRunEmptySentence(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
		s.writeSentence(t)
		s.writeSentence(t, "!re", "=address=1.2.3.4/32")
		s.writeSentence(t, "!done")
	}()

	sen, err := c.Run("/ip/address")
	require.NoError(t, err)
	require.Equal(t, "!re @ [{`address` `1.2.3.4/32`}]\n!done @ []", sen.String(), "/ip/address")
}

func TestRunEOF(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
	}()

	_, err := c.Run("/ip/address")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorIs(t, err, io.EOF)
}

func TestRunInvalidSentence(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
		s.writeSentence(t, "!xxx")
	}()

	_, err := c.Run("/ip/address")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorContains(t, err, "unknown RouterOS reply word: !xxx")
}

func TestRunTrap(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
		s.writeSentence(t, "!trap", "=message=Some device error message")
		s.writeSentence(t, "!done")
	}()

	_, err := c.Run("/ip/address")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorContains(t, err, "from RouterOS device: Some device error message")
}

func TestRunTrapWithoutMessage(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
		s.writeSentence(t, "!trap", "=some=unknown key")
		s.writeSentence(t, "!done")
	}()

	_, err := c.Run("/ip/address")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorContains(t, err, "from RouterOS device: unknown error: !trap @ [{`some` `unknown key`}]")
}

func TestRunFatal(t *testing.T) {
	c, s := newPair(t)
	defer c.Close()

	go func() {
		defer s.Close()
		s.readSentence(t, "/ip/address @ []")
		s.writeSentence(t, "!fatal", "=message=Some device error message")
	}()

	_, err := c.Run("/ip/address")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorContains(t, err, "from RouterOS device: Some device error message")
}

func TestRunAfterClose(t *testing.T) {
	c, s := newPair(t)
	c.Close()
	s.Close()

	_, err := c.Run("/ip/address")
	require.Error(t, err, "Run succeeded; want error")
	require.ErrorContains(t, err, "endsentence error: io: read/write on closed pipe")
}

type conn struct {
	*io.PipeReader
	*io.PipeWriter
}

func (c *conn) Close() error {
	c.PipeReader.Close()
	c.PipeWriter.Close()
	return nil
}

func newPair(t *testing.T) (*routeros.Client, *fakeServer) {
	t.Helper()

	ar, aw := io.Pipe()
	br, bw := io.Pipe()

	c, err := routeros.NewClient(&conn{ar, bw})
	require.NoError(t, err)

	s := &fakeServer{
		proto.NewReader(br),
		proto.NewWriter(aw),
		&conn{br, aw},
	}

	return c, s
}

type fakeServer struct {
	r proto.Reader
	w proto.Writer
	io.Closer
}

func (f *fakeServer) readSentence(t *testing.T, want string) {
	sen, err := f.r.ReadSentence()
	require.NoError(t, err)
	require.Equal(t, want, sen.String())
	t.Logf("< %s\n", sen)
}

func (f *fakeServer) writeSentence(t *testing.T, sentence ...string) {
	t.Logf("> %#q\n", sentence)
	f.w.BeginSentence()

	for _, word := range sentence {
		f.w.WriteWord(word)
	}

	err := f.w.EndSentence()
	require.NoError(t, err)
}
