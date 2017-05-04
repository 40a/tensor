package util

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	dat, _ := ioutil.ReadFile("../VERSION")

	str := strings.Split(string(dat), " ")
	assert.Equal(t, Version, str[0], "Version should be equal")
}
