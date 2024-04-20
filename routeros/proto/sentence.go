package proto

import "fmt"

// Sentence is a line read from a RouterOS device.
type Sentence struct {
	Map  map[string]string
	Word string
	Tag  string
}

type Pair struct {
	Key, Value string
}

func (p *Pair) String() string {
	return p.Key + "=" + p.Value
}

func NewSentence() *Sentence {
	return &Sentence{
		Map: make(map[string]string),
	}
}

func (sen *Sentence) String() string {
	return fmt.Sprintf("%s @%s %#q", sen.Word, sen.Tag, sen.AsList())
}

func (sen *Sentence) AsList() []Pair {
	res := make([]Pair, 0, len(sen.Map))

	for k, v := range sen.Map {
		res = append(res, Pair{k, v})
	}

	return res
}
