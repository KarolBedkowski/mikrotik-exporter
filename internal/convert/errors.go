package convert

//
// errors.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

import "errors"

var (
	ErrEmptyValue      = errors.New("empty value")
	ErrInvalidDuration = errors.New("invalid duration value sent to regex")
)

// ----------------------------------------------------------------------------

type InvalidInputError string

func (i InvalidInputError) Error() string {
	return "invalid input: " + string(i)
}
