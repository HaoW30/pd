// Copyright 2017 TiKV Project Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/kvproto/pkg/pdpb"

	"github.com/tikv/pd/pkg/core"
	"github.com/tikv/pd/pkg/statistics"
	"github.com/tikv/pd/pkg/utils/apiutil"
	"github.com/tikv/pd/pkg/utils/keypath"
	"github.com/tikv/pd/tests"
)

type statTestSuite struct {
	suite.Suite
	env *tests.SchedulingTestEnvironment
}

func TestStatTestSuite(t *testing.T) {
	suite.Run(t, new(statTestSuite))
}

func (suite *statTestSuite) SetupSuite() {
	suite.env = tests.NewSchedulingTestEnvironment(suite.T())
}

func (suite *statTestSuite) TearDownSuite() {
	suite.env.Cleanup()
}

func (suite *statTestSuite) TestRegionStats() {
	suite.env.RunTestInNonMicroserviceEnv(suite.checkRegionStats)
}

func (suite *statTestSuite) checkRegionStats(cluster *tests.TestCluster) {
	re := suite.Require()
	leader := cluster.GetLeaderServer()
	urlPrefix := leader.GetAddr() + "/pd/api/v1"

	statsURL := urlPrefix + "/stats/region"
	epoch := &metapb.RegionEpoch{
		ConfVer: 1,
		Version: 1,
	}
	intervalSec := uint64(100)
	regions := []*core.RegionInfo{
		core.NewRegionInfo(&metapb.Region{
			Id:       1,
			StartKey: []byte(""),
			EndKey:   []byte("a"),
			Peers: []*metapb.Peer{
				{Id: 101, StoreId: 1},
				{Id: 102, StoreId: 2},
				{Id: 103, StoreId: 3},
			},
			RegionEpoch: epoch,
		},
			&metapb.Peer{Id: 101, StoreId: 1},
			core.SetReportInterval(0, intervalSec),
			core.SetApproximateSize(100),
			core.SetApproximateKvSize(80),
			core.SetApproximateKeys(50),
			core.SetWrittenBytes(50000*intervalSec),
			core.SetWrittenKeys(5000*intervalSec),
			core.SetWrittenQuery(500*intervalSec),
		),
		core.NewRegionInfo(
			&metapb.Region{
				Id:       2,
				StartKey: []byte("a"),
				EndKey:   []byte("t"),
				Peers: []*metapb.Peer{
					{Id: 104, StoreId: 1},
					{Id: 105, StoreId: 4},
					{Id: 106, StoreId: 5},
				},
				RegionEpoch: epoch,
			},
			&metapb.Peer{Id: 105, StoreId: 4},
			core.SetApproximateSize(200),
			core.SetApproximateKvSize(180),
			core.SetApproximateKeys(150),
		),
		core.NewRegionInfo(
			&metapb.Region{
				Id:       3,
				StartKey: []byte("t"),
				EndKey:   []byte("x"),
				Peers: []*metapb.Peer{
					{Id: 106, StoreId: 1},
					{Id: 107, StoreId: 5},
				},
				RegionEpoch: epoch,
			},
			&metapb.Peer{Id: 107, StoreId: 5},
			core.SetApproximateSize(1),
			core.SetApproximateKvSize(1),
			core.SetApproximateKeys(1),
		),
		core.NewRegionInfo(
			&metapb.Region{
				Id:       4,
				StartKey: []byte("x"),
				EndKey:   []byte(""),
				Peers: []*metapb.Peer{
					{Id: 108, StoreId: 4},
				},
				RegionEpoch: epoch,
			},
			&metapb.Peer{Id: 108, StoreId: 4},
			core.SetApproximateSize(50),
			core.SetApproximateKvSize(30),
			core.SetApproximateKeys(20),
		),
	}

	for range 5 {
		for _, r := range regions {
			tests.MustPutRegionInfo(re, cluster, r)
		}
	}

	// Distribution (L for leader, F for follower):
	// region range       size  rows store1 store2 store3 store4 store5
	// 1      ["", "a")   100   50 	  L      F      F
	// 2      ["a", "t")  200   150	  F                    L      F
	// 3      ["t", "x")  1     1	  F                           L
	// 4      ["x", "")   50    20                   	   L

	statsAll := &statistics.RegionStats{
		Count:            4,
		EmptyCount:       1,
		StorageSize:      351,
		UserStorageSize:  291,
		StorageKeys:      221,
		StoreLeaderCount: map[uint64]int{1: 1, 4: 2, 5: 1},
		StorePeerCount:   map[uint64]int{1: 3, 2: 1, 3: 1, 4: 2, 5: 2},
		StoreLeaderSize:  map[uint64]int64{1: 100, 4: 250, 5: 1},
		StoreLeaderKeys:  map[uint64]int64{1: 50, 4: 170, 5: 1},
		StorePeerSize:    map[uint64]int64{1: 301, 2: 100, 3: 100, 4: 250, 5: 201},
		StorePeerKeys:    map[uint64]int64{1: 201, 2: 50, 3: 50, 4: 170, 5: 151},
	}

	stats23 := &statistics.RegionStats{
		Count:            2,
		EmptyCount:       1,
		StorageSize:      201,
		UserStorageSize:  181,
		StorageKeys:      151,
		StoreLeaderCount: map[uint64]int{4: 1, 5: 1},
		StorePeerCount:   map[uint64]int{1: 2, 4: 1, 5: 2},
		StoreLeaderSize:  map[uint64]int64{4: 200, 5: 1},
		StoreLeaderKeys:  map[uint64]int64{4: 150, 5: 1},
		StorePeerSize:    map[uint64]int64{1: 201, 4: 200, 5: 201},
		StorePeerKeys:    map[uint64]int64{1: 151, 4: 150, 5: 151},
	}

	testdata := []struct {
		startKey string
		endKey   string
		expect   *statistics.RegionStats
	}{
		{
			startKey: "",
			endKey:   "",
			expect:   statsAll,
		}, {
			startKey: url.QueryEscape("\x01\x02"),
			endKey:   url.QueryEscape("xyz\x00\x00"),
			expect:   statsAll,
		},
		{
			startKey: url.QueryEscape("a"),
			endKey:   url.QueryEscape("x"),
			expect:   stats23,
		},
	}

	for _, data := range testdata {
		for _, query := range []string{"", "count"} {
			args := fmt.Sprintf("?start_key=%s&end_key=%s&%s", data.startKey, data.endKey, query)
			res, err := tests.TestDialClient.Get(statsURL + args)
			re.NoError(err)
			stats := &statistics.RegionStats{}
			err = apiutil.ReadJSON(res.Body, stats)
			re.NoError(res.Body.Close())
			re.NoError(err)
			re.Equal(data.expect.Count, stats.Count)
			if query != "count" {
				re.Equal(data.expect, stats)
			}
		}
	}

	hotStats := &statistics.RegionStats{
		Count:                4,
		EmptyCount:           1,
		StorageSize:          351,
		UserStorageSize:      291,
		StorageKeys:          221,
		StoreLeaderCount:     map[uint64]int{1: 1},
		StorePeerCount:       map[uint64]int{1: 3},
		StoreLeaderSize:      map[uint64]int64{1: 100},
		StoreLeaderKeys:      map[uint64]int64{1: 50},
		StorePeerSize:        map[uint64]int64{1: 301},
		StorePeerKeys:        map[uint64]int64{1: 201},
		StoreWriteBytes:      map[uint64]uint64{1: regions[0].GetBytesWritten() / intervalSec},
		StoreWriteKeys:       map[uint64]uint64{1: regions[0].GetKeysWritten() / intervalSec},
		StoreWriteQuery:      map[uint64]uint64{1: regions[0].GetWriteQueryNum() / intervalSec},
		StoreLeaderReadBytes: map[uint64]uint64{1: 10000},
		StoreLeaderReadKeys:  map[uint64]uint64{1: 1000},
		StoreLeaderReadQuery: map[uint64]uint64{1: 100},
		StorePeerReadBytes:   map[uint64]uint64{1: 10000},
		StorePeerReadKeys:    map[uint64]uint64{1: 1000},
		StorePeerReadQuery:   map[uint64]uint64{1: 100},
		StoreEngine:          map[uint64]string{1: core.EngineTiKV},
	}

	storeReq := pdpb.StoreHeartbeatRequest{
		Header: &pdpb.RequestHeader{ClusterId: keypath.ClusterID()},
		Stats: &pdpb.StoreStats{
			StoreId:  1,
			Interval: &pdpb.TimeInterval{StartTimestamp: 0, EndTimestamp: 10},
			PeerStats: []*pdpb.PeerStat{
				{
					RegionId:  1,
					ReadBytes: 10000 * 10,
					ReadKeys:  1000 * 10,
					QueryStats: &pdpb.QueryStats{
						Get: 100 * 10,
					},
				},
			},
		},
	}

	tests.MustHandleStoreHeartbeat(re, cluster, &storeReq)

	args := fmt.Sprintf("?use_hot&start_key=%s&end_key=%s&engine=tikv", "", "")
	stats := &statistics.RegionStats{}
	res, err := tests.TestDialClient.Get(statsURL + args)
	re.NoError(err)
	err = apiutil.ReadJSON(res.Body, stats)
	re.NoError(res.Body.Close())
	re.NoError(err)
	re.Equal(hotStats, stats)
}
