package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionVariableIsSet(t *testing.T) {
	assert.NotEmpty(t, version, "version variable should not be empty")
}
