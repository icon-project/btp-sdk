package bmc

import (
	"fmt"
	"testing"
	"time"

	"github.com/icon-project/btp2/common/log"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/database"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/tracker"
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
				NetworkIcon: "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4NDQo8c3ZnIHhtbG5zPSJodHRw\nOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgdmlld0JveD0iMCAwIDI5IDI5Ij48ZGVmcz48c3R5bGU+\nLmE1MTI4Mjg3LTFiMmYtNGFhNy1iZjgzLWUzZDBhMmQ3MTljN3tmaWxsOiMxOGEwYjE7fTwvc3R5\nbGU+PC9kZWZzPjx0aXRsZT5pY29uPC90aXRsZT48ZyBpZD0iYWE4ZDcyZjMtZTkxZC00NjY4LTg2\nNGYtZjZjY2QzMmFiNWVlIiBkYXRhLW5hbWU9IkNhbHF1ZSAyIj48ZyBpZD0iZjQ4NDM3ZWUtZGYy\nZS00ZDFmLTgxNTUtYTUxYjk0NWYzNGVjIiBkYXRhLW5hbWU9IkxpbmUiPjxnIGlkPSJmYjU2YTY3\nZi0xYTFiLTRmY2ItYWZhZi01YmRmYjdjMjBiOTIiIGRhdGEtbmFtZT0iaWNvbiI+PGNpcmNsZSBj\nbGFzcz0iYTUxMjgyODctMWIyZi00YWE3LWJmODMtZTNkMGEyZDcxOWM3IiBjeD0iMjYuNSIgY3k9\nIjIuNSIgcj0iMi41Ii8+PHBhdGggY2xhc3M9ImE1MTI4Mjg3LTFiMmYtNGFhNy1iZjgzLWUzZDBh\nMmQ3MTljNyIgZD0iTTcuMjgsMTguNzlBOC4zNyw4LjM3LDAsMCwxLDE4Ljc0LDcuMzNsMi0yLS4x\nLS4wOEExMS4xNCwxMS4xNCwwLDAsMCw1LjI4LDIwLjc5WiIvPjxwYXRoIGNsYXNzPSJhNTEyODI4\nNy0xYjJmLTRhYTctYmY4My1lM2QwYTJkNzE5YzciIGQ9Ik0yMS43MiwxMC4zMkE4LjM3LDguMzcs\nMCwwLDEsMTAuMjYsMjEuNzhsLTIsMkExMS4xMywxMS4xMywwLDAsMCwyMy43Miw4LjMyWiIvPjxj\naXJjbGUgY2xhc3M9ImE1MTI4Mjg3LTFiMmYtNGFhNy1iZjgzLWUzZDBhMmQ3MTljNyIgY3g9IjIu\nNSIgY3k9IjI2LjUiIHI9IjIuNSIvPjwvZz48L2c+PC9nPjwvc3ZnPg==",
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
				NetworkIcon: "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAzMjAg\nNTEyIj48IS0tISBGb250IEF3ZXNvbWUgUHJvIDYuNC4yIGJ5IEBmb250YXdlc29tZSAtIGh0dHBz\nOi8vZm9udGF3ZXNvbWUuY29tIExpY2Vuc2UgLSBodHRwczovL2ZvbnRhd2Vzb21lLmNvbS9saWNl\nbnNlIChDb21tZXJjaWFsIExpY2Vuc2UpIENvcHlyaWdodCAyMDIzIEZvbnRpY29ucywgSW5jLiAt\nLT48cGF0aCBkPSJNMzExLjkgMjYwLjhMMTYwIDM1My42IDggMjYwLjggMTYwIDBsMTUxLjkgMjYw\nLjh6TTE2MCAzODMuNEw4IDI5MC42IDE2MCA1MTJsMTUyLTIyMS40LTE1MiA5Mi44eiIvPjwvc3Zn\nPg==",
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
				NetworkIcon: "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4NCjwhLS0gR2VuZXJhdG9yOiBB\nZG9iZSBJbGx1c3RyYXRvciAyNi4wLjMsIFNWRyBFeHBvcnQgUGx1Zy1JbiAuIFNWRyBWZXJzaW9u\nOiA2LjAwIEJ1aWxkIDApICAtLT4NCjxzdmcgdmVyc2lvbj0iMS4wIiBpZD0ia2F0bWFuXzEiIHht\nbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgeG1sbnM6eGxpbms9Imh0dHA6Ly93d3cu\ndzMub3JnLzE5OTkveGxpbmsiIHg9IjBweCIgeT0iMHB4Ig0KCSB2aWV3Qm94PSIwIDAgODAwIDYw\nMCIgc3R5bGU9ImVuYWJsZS1iYWNrZ3JvdW5kOm5ldyAwIDAgODAwIDYwMDsiIHhtbDpzcGFjZT0i\ncHJlc2VydmUiPg0KPHN0eWxlIHR5cGU9InRleHQvY3NzIj4NCgkuc3Qwe2ZpbGwtcnVsZTpldmVu\nb2RkO2NsaXAtcnVsZTpldmVub2RkO2ZpbGw6I0YzQkEyRjt9DQoJLnN0MXtmaWxsLXJ1bGU6ZXZl\nbm9kZDtjbGlwLXJ1bGU6ZXZlbm9kZDtmaWxsOiMxMzE0MTU7fQ0KPC9zdHlsZT4NCjxnIGlkPSJM\naWdodCI+DQoJPGcgaWQ9Ik9uZUFydC1feDIwMjJfLURlc2t0b3AtX3gyMDIyXy1MaWdodCIgdHJh\nbnNmb3JtPSJ0cmFuc2xhdGUoLTQ1Ny4wMDAwMDAsIC0xNTE1LjAwMDAwMCkiPg0KCQk8ZyBpZD0i\nQmxvY2siIHRyYW5zZm9ybT0idHJhbnNsYXRlKDQxLjAwMDAwMCwgMTI2My4wMDAwMDApIj4NCgkJ\nCTxnIGlkPSJUVkwiIHRyYW5zZm9ybT0idHJhbnNsYXRlKDQ4LjAwMDAwMCwgMjUyLjAwMDAwMCki\nPg0KCQkJCTxnIGlkPSJJY29uc194MkZfSWNvbi0yNF94MkZfY2FrZSIgdHJhbnNmb3JtPSJ0cmFu\nc2xhdGUoMzY4LjAwMDAwMCwgMC4wMDAwMDApIj4NCgkJCQkJPGNpcmNsZSBpZD0iT3ZhbCIgY2xh\nc3M9InN0MCIgY3g9IjM5OS44IiBjeT0iMjk5LjYiIHI9IjIzMC43Ii8+DQoJCQkJCTxnIGlkPSJJ\nY29uc194MkZfaWNvbi0yNF94MkZfbmV0d29ya3NfeDJGX2JpbmFuY2VfeDVGX3NtYXJ0X3g1Rl9j\naGFpbiIgdHJhbnNmb3JtPSJ0cmFuc2xhdGUoMy4zMzMzMzMsIDMuMzMzMzMzKSI+DQoJCQkJCQk8\ncGF0aCBpZD0iQ29tYmluZWQtU2hhcGUiIGNsYXNzPSJzdDEiIGQ9Ik00NTYuMywzMjAuOGwzNC44\nLDM0LjdMMzk2LjUsNDUwTDMwMiwzNTUuNWwzNC44LTM0LjdsNTkuNyw1OS43TDQ1Ni4zLDMyMC44\neg0KCQkJCQkJCSBNMzk2LjUsMjYxbDM1LjMsMzUuM2gwbDAsMGwtMzUuMywzNS4zbC0zNS4yLTM1\nLjJsMC0wLjFsMCwwbDYuMi02LjJsMy0zTDM5Ni41LDI2MXogTTI3Ny41LDI2MS41bDM0LjgsMzQu\nOEwyNzcuNSwzMzENCgkJCQkJCQlsLTM0LjgtMzQuOEwyNzcuNSwyNjEuNXogTTUxNS41LDI2MS41\nbDM0LjgsMzQuOEw1MTUuNSwzMzFsLTM0LjgtMzQuOEw1MTUuNSwyNjEuNXogTTM5Ni41LDE0Mi41\nTDQ5MSwyMzdsLTM0LjgsMzQuOA0KCQkJCQkJCUwzOTYuNSwyMTJsLTU5LjcsNTkuN0wzMDIsMjM3\nTDM5Ni41LDE0Mi41eiIvPg0KCQkJCQk8L2c+DQoJCQkJPC9nPg0KCQkJPC9nPg0KCQk8L2c+DQoJ\nPC9nPg0KPC9nPg0KPC9zdmc+DQo=",
			},
		},
	}
	Database = database.Config{
		Driver: "mysql",
		User: "test",
		Password: "test1234",
		Host: "127.0.0.1",
		Port: 3306,
		DBName: "btp_sdk",
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

	db, _ := database.OpenDatabase(Database, l)

	tkr := NewTestTracker(t, s, db, l)
	fmt.Println("Tracker Name: ", tkr.Name())

	err := tkr.Start()
	if err != nil {
		assert.FailNow(t, "fail to monitor event", err)
	}
	<-time.After(time.Minute * 10)
}

func NewTestTracker(t *testing.T, s service.Service, db *gorm.DB, l log.Logger) tracker.Tracker {
	networks := make(map[string]tracker.Network)
	for network, config := range configs {
		networks[network] = tracker.Network{
			NetworkType: config.NetworkType,
			Adaptor:     adaptor(t, network),
			Options:     MustEncodeOptions(config.TrackerOption),
		}
	}
	tkr, err := tracker.NewTracker(bmc.ServiceName, s, networks, db, l)
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
