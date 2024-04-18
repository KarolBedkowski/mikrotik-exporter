package proto

import "fmt"

//
// error.go
//

type InvalidSentenceWordError struct {
	word []byte
}

func (e InvalidSentenceWordError) Error() string {
	return fmt.Sprintf("invalid RouterOS sentence word: %#q", e.word)
}
