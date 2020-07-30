package hraftd_test

import (
	"fmt"
	"testing"

	"github.com/bingoohuang/hraftd"

	"github.com/stretchr/testify/assert"
)

func TestHostIP(t *testing.T) {
	hostIP := hraftd.InferHostIPv4("")
	fmt.Println("HostIP:", hostIP)
	assert.NotEmpty(t, hostIP)
}
