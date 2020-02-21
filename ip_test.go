package hraftd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostIP(t *testing.T) {
	hostIP := InferHostIPv4("")
	fmt.Println("HostIP:", hostIP)
	assert.NotEmpty(t, hostIP)
}
