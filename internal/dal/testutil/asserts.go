package testutil

import (
	"github.com/stretchr/testify/assert"
)

func AssertErrorIsAndContains(wantErr error, contains string) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, err error, i ...interface{}) bool {
		return assert.Error(t, err, i...) && assert.ErrorIs(t, err, wantErr) && assert.ErrorContains(t, err, contains)
	}
}
