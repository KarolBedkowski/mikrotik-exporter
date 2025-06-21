package metrics

import "fmt"

//
// errors.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

type BuilderError string

func newBuilderError(format string, v ...any) BuilderError {
	return BuilderError(fmt.Sprintf(format, v...))
}

func (b BuilderError) Error() string {
	return string(b)
}

// --------------------------------------------
