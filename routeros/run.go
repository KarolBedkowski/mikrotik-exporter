package routeros

import "fmt"

// Run simply calls RunArgs().
func (c *Client) Run(sentence ...string) (*Reply, error) {
	return c.RunArgs(sentence)
}

// RunArgs sends a sentence to the RouterOS device and waits for the reply.
func (c *Client) RunArgs(sentence []string) (*Reply, error) {
	c.w.BeginSentence()

	for _, word := range sentence {
		c.w.WriteWord(word)
	}

	return c.endCommandSync()
}

func (c *Client) endCommandSync() (*Reply, error) {
	if err := c.w.EndSentence(); err != nil {
		return nil, fmt.Errorf("endsentence error: %w", err)
	}

	return c.readReply()
}
