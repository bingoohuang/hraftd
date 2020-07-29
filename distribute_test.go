package hraftd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type dataItem struct {
	iid string
}

func (d dataItem) ID() string {
	return d.iid
}

type dataItemWrap struct {
	dataItem
	NodeID
}

func (d *dataItemWrap) Distribute(nodeID NodeID, item Identifier) {
	d.NodeID = nodeID
	d.dataItem = item.(dataItem)
}

// nolint:funlen
func TestDistribute(t *testing.T) {
	d := NewDistributor()
	peers := []Peer{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	data := []dataItem{{iid: "1"}, {iid: "2"}, {iid: "3"}}
	assert.Equal(t, []dataItemWrap{
		{NodeID: "1", dataItem: dataItem{iid: "1"}},
		{NodeID: "2", dataItem: dataItem{iid: "2"}},
		{NodeID: "3", dataItem: dataItem{iid: "3"}},
	}, d.Distribute(peers, data, dataItemWrap{}).([]dataItemWrap))
	assert.Equal(t, map[string]NodeID{"1": "1", "2": "2", "3": "3"}, d.stickyMap)

	data = []dataItem{{iid: "1"}, {iid: "2"}, {iid: "3"}, {iid: "4"}}
	assert.Equal(t, []dataItemWrap{
		{NodeID: "1", dataItem: dataItem{iid: "1"}},
		{NodeID: "2", dataItem: dataItem{iid: "2"}},
		{NodeID: "3", dataItem: dataItem{iid: "3"}},
		{NodeID: "1", dataItem: dataItem{iid: "4"}},
	}, d.Distribute(peers, data, dataItemWrap{}).([]dataItemWrap))
	assert.Equal(t, map[string]NodeID{"1": "1", "4": "1", "2": "2", "3": "3"}, d.stickyMap)

	data = []dataItem{{iid: "1"}, {iid: "2"}, {iid: "3"}, {iid: "4"}, {iid: "5"}, {iid: "6"}}
	assert.Equal(t, []dataItemWrap{
		{NodeID: "1", dataItem: dataItem{iid: "1"}},
		{NodeID: "2", dataItem: dataItem{iid: "2"}},
		{NodeID: "3", dataItem: dataItem{iid: "3"}},
		{NodeID: "1", dataItem: dataItem{iid: "4"}},
		{NodeID: "2", dataItem: dataItem{iid: "5"}},
		{NodeID: "3", dataItem: dataItem{iid: "6"}},
	}, d.Distribute(peers, data, dataItemWrap{}).([]dataItemWrap))

	data = []dataItem{{iid: "1"}, {iid: "7"}, {iid: "3"}, {iid: "4"}, {iid: "5"}, {iid: "6"}}
	assert.Equal(t, []dataItemWrap{
		{NodeID: "1", dataItem: dataItem{iid: "1"}},
		{NodeID: "3", dataItem: dataItem{iid: "3"}},
		{NodeID: "1", dataItem: dataItem{iid: "4"}},
		{NodeID: "2", dataItem: dataItem{iid: "5"}},
		{NodeID: "3", dataItem: dataItem{iid: "6"}},
		{NodeID: "2", dataItem: dataItem{iid: "7"}},
	}, d.Distribute(peers, data, dataItemWrap{}).([]dataItemWrap))

	// 测试重新分配
	peers = []Peer{{ID: "1"}, {ID: "3"}}
	data = []dataItem{{iid: "1"}, {iid: "7"}, {iid: "3"}, {iid: "4"}, {iid: "5"}, {iid: "6"}}
	assert.Equal(t, []dataItemWrap{
		{NodeID: "1", dataItem: dataItem{iid: "1"}},
		{NodeID: "3", dataItem: dataItem{iid: "3"}},
		{NodeID: "1", dataItem: dataItem{iid: "4"}},
		{NodeID: "3", dataItem: dataItem{iid: "6"}},
		{NodeID: "1", dataItem: dataItem{iid: "7"}},
		{NodeID: "3", dataItem: dataItem{iid: "5"}},
	}, d.Distribute(peers, data, dataItemWrap{}).([]dataItemWrap))
	assert.Equal(t, map[string]NodeID{"1": "1", "3": "3", "4": "1", "6": "3", "7": "1", "5": "3"}, d.stickyMap)

	peers = []Peer{{ID: "1"}}
	data = []dataItem{{iid: "1"}, {iid: "7"}, {iid: "3"}, {iid: "4"}, {iid: "5"}, {iid: "6"}}
	assert.Equal(t, []dataItemWrap{
		{NodeID: "1", dataItem: dataItem{iid: "1"}},
		{NodeID: "1", dataItem: dataItem{iid: "7"}},
		{NodeID: "1", dataItem: dataItem{iid: "4"}},
		{NodeID: "1", dataItem: dataItem{iid: "3"}},
		{NodeID: "1", dataItem: dataItem{iid: "5"}},
		{NodeID: "1", dataItem: dataItem{iid: "6"}},
	}, d.Distribute(peers, data, dataItemWrap{}).([]dataItemWrap))

	assert.Equal(t, map[string]NodeID{"1": "1", "7": "1", "4": "1", "3": "1", "5": "1", "6": "1"}, d.stickyMap)
	assert.Equal(t, []dataItemWrap{}, d.Distribute(peers, []dataItem{}, dataItemWrap{}).([]dataItemWrap))
	assert.Equal(t, map[string]NodeID{"1": "1", "7": "1", "4": "1", "3": "1", "5": "1", "6": "1"}, d.stickyMap)

	assert.Panics(t, func() { d.Distribute(peers, d, dataItemWrap{}) })
	assert.Panics(t, func() { d.Distribute(peers, []string{}, dataItemWrap{}) })
}
