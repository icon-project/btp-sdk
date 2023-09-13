package bmc

import (
	"fmt"
	"testing"
	"time"

	"github.com/icon-project/btp2/common/log"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/btptracker"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/utils"
)

const (
	networkIconTest = "icon_test"
	networkEthTest  = "eth_test"
	networkBscTest  = "bsc_test"
)

var (
	configs = map[string]TestConfig{
		networkIconTest: {
			Endpoint:    "http://20.20.0.32:19080/api/v3/icon_dex",
			NetworkType: icon.NetworkTypeIcon,
			AdaptorOption: icon.AdaptorOption{
				NetworkID: "0x3",
				//TransportLogLevel: contract.LogLevel(log.DebugLevel),
			},
			ServiceOption: service.MultiContractServiceOption{
				service.MultiContractServiceOptionNameDefault: "cx2093dd8134f26df11aa928c86b6f5bac64a1cf83",
			},
			TrackerOption: TrackerOptions{
				InitHeight:     3465818,
				NetworkAddress: "0x3.icon",
			},
		},
		networkEthTest: {
			Endpoint:    "http://20.20.0.32:8545",
			NetworkType: eth.NetworkTypeEth,
			AdaptorOption: eth.AdaptorOption{
				FinalityMonitor: MustEncodeOptions(eth.FinalityMonitorOptions{
					PollingPeriodSec: 3,
				}),
			},
			ServiceOption: service.MultiContractServiceOption{
				service.MultiContractServiceOptionNameDefault: "0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9",
			},
			TrackerOption: TrackerOptions{
				InitHeight:     22088,
				NetworkAddress: "0x25d6efd.eth2",
			},
		},
		networkBscTest: {
			Endpoint:    "http://20.20.0.32:8546",
			NetworkType: eth.NetworkTypeBSC,
			AdaptorOption: eth.AdaptorOption{
				FinalityMonitor: MustEncodeOptions(eth.FinalityMonitorOptions{
					PollingPeriodSec: 3,
				}),
			},
			ServiceOption: service.MultiContractServiceOption{
				service.MultiContractServiceOptionNameDefault: "0x6d80E9654e409c8deE65f64f3a029962682940fe",
			},
			TrackerOption: TrackerOptions{
				InitHeight:     3099,
				NetworkAddress: "0x63.bsc",
			},
		},
	}
	storageConfig = &utils.StorageConfig{
		DBType:   "mysql",
		HostName: "127.0.0.1:3306",
		DBName:   "btp_sdk",
		UserName: "test",
		Password: "test1234",
	}
)

func MustEncodeOptions(v interface{}) contract.Options {
	opt, err := contract.EncodeOptions(v)
	if err != nil {
		log.Panicf("%+v", err)
	}
	return opt
}

type TestConfig struct {
	NetworkType   string
	Endpoint      string
	AdaptorOption interface{}
	ServiceOption service.MultiContractServiceOption
	TrackerOption TrackerOptions
}

func adaptor(t *testing.T, networkType string) contract.Adaptor {
	config := configs[networkType]
	opt, err := contract.EncodeOptions(config.AdaptorOption)
	if err != nil {
		assert.FailNow(t, "fail to EncodeOptions", err)
	}
	l := log.GlobalLogger()
	a, err := contract.NewAdaptor(config.NetworkType, config.Endpoint, opt, l)
	if err != nil {
		assert.FailNow(t, "fail to NewAdaptor", err)
	}
	return a
}

func Test_Tracker(t *testing.T) {
	l := log.GlobalLogger()

	s := NewTestService(t, l)
	fmt.Println("Service Name: ", s.Name())

	db, _ := utils.NewStorage(storageConfig)

	tkr := NewTestTracker(t, s, db, l)
	fmt.Println("Tracker Name: ", tkr.Name())

	err := tkr.Start()
	if err != nil {
		assert.FailNow(t, "fail to monitor event", err)
	}
	<-time.After(time.Minute * 10)
}

func NewTestTracker(t *testing.T, s service.Service, db *gorm.DB, l log.Logger) btptracker.Tracker {
	networks := make(map[string]btptracker.Network)
	for network, config := range configs {
		networks[network] = btptracker.Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.TrackerOption),
		}
	}
	tkr, err := btptracker.NewTracker(bmc.ServiceName, s, networks, db, l)
	if err != nil {
		assert.FailNow(t, "fail to NewTracker", err)
	}
	assert.Equal(t, bmc.ServiceName, tkr.Name())
	return tkr
}

func NewTestService(t *testing.T, l log.Logger) service.Service {
	networks := make(map[string]service.Network)
	for network, config := range configs {
		networks[network] = service.Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.ServiceOption),
		}
	}
	s, err := service.NewService(bmc.ServiceName, networks, l)
	if err != nil {
		assert.FailNow(t, "fail to NewService", err)
	}
	assert.Equal(t, bmc.ServiceName, s.Name())
	return s
}
