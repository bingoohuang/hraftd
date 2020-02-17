package hraftd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type InputStruct struct{ Name string }
type OutputStruct struct{ Name string }

func TestDealer_Invoke(t *testing.T) {
	dm := MakeDealerMap()

	_ = dm.RegisterJobDealer("/testjob1", func(is InputStruct) (OutputStruct, error) {
		return OutputStruct(is), nil
	})

	dealer := dm.Dealers["/testjob1"]
	ret, err := dealer.Invoke([]byte(`{"Name":"bingoo"}`))
	assert.Nil(t, err)
	assert.Equal(t, OutputStruct{Name: "bingoo"}, ret)

	_ = dm.RegisterJobDealer("/testjob2", func(is *InputStruct) (*OutputStruct, error) {
		return &OutputStruct{Name: is.Name}, nil
	})

	dealer = dm.Dealers["/testjob2"]
	ret, err = dealer.Invoke([]byte(`{"Name":"bingoo"}`))
	assert.Nil(t, err)
	assert.Equal(t, OutputStruct{Name: "bingoo"}, *(ret.(*OutputStruct)))

	_ = dm.RegisterJobDealer("/testjob3", func(is InputStruct) (*OutputStruct, error) {
		return nil, errors.New("error occurred")
	})

	dealer = dm.Dealers["/testjob3"]
	ret, err = dealer.Invoke([]byte(`{"Name":"bingoo"}`))
	assert.Nil(t, ret)
	assert.Equal(t, "error occurred", err.Error())

	_ = dm.RegisterJobDealer("/testjob4", func(is InputStruct) (*OutputStruct, error) {
		return nil, nil
	})

	dealer = dm.Dealers["/testjob4"]
	ret, err = dealer.Invoke([]byte(`{"Name":"bingoo"}`))
	assert.Nil(t, ret)
	assert.Nil(t, err)

	_ = dm.RegisterJobDealer("/testjob5", func(is []InputStruct) ([]OutputStruct, error) {
		os := make([]OutputStruct, len(is))

		for i, item := range is {
			os[i] = OutputStruct(item)
		}

		return os, nil
	})

	dealer = dm.Dealers["/testjob5"]
	ret, err = dealer.Invoke([]byte(`[{"Name":"bingoo"}]`))
	assert.Nil(t, err)
	assert.Equal(t, []OutputStruct{{Name: "bingoo"}}, ret)
}
