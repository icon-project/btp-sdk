package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/icon-project/btp2/common/cli"
	"github.com/icon-project/btp2/common/config"
	"github.com/icon-project/btp2/common/log"
	"github.com/icon-project/btp2/common/wallet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/icon-project/btp-sdk/api"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/contract/eth"
	"github.com/icon-project/btp-sdk/contract/icon"
	"github.com/icon-project/btp-sdk/service"
	"github.com/icon-project/btp-sdk/service/bmc"
	"github.com/icon-project/btp-sdk/service/xcall"
)

type Config struct {
	config.FileConfig `json:",squash"`

	Server   ServerConfig             `json:"server"`
	Networks map[string]NetworkConfig `json:"networks"`

	LogLevel     string            `json:"log_level"`
	ConsoleLevel string            `json:"console_level"`
	LogWriter    *log.WriterConfig `json:"log_writer,omitempty"`
}

type ServerConfig struct {
	Address      string `json:"address"`
	DumpLogLevel string `json:"dump_log_level,omitempty"`
}

type NetworkConfig struct {
	NetworkType string                      `json:"type"`
	Endpoint    string                      `json:"endpoint"`
	Options     contract.Options            `json:"options,omitempty"`
	Services    map[string]contract.Options `json:"services,omitempty"`
	Signer      *SignerConfig               `json:"signer,omitempty"`
}

type SignerConfig struct {
	Keystore string `json:"keystore"`
	Secret   string `json:"secret"`
}

func ReadConfig(filePath string, cfg *Config, vc *viper.Viper) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("fail to open config file=%s err=%+v", filePath, err)
	}
	vc.SetConfigType("json")
	err = vc.ReadConfig(f)
	if err != nil {
		return fmt.Errorf("fail to read config file=%s err=%+v", filePath, err)
	}
	if err = vc.Unmarshal(cfg, cli.ViperDecodeOptJson); err != nil {
		return fmt.Errorf("fail to unmarshall config from env err=%+v", err)
	}
	cfg.FilePath, _ = filepath.Abs(filePath)
	return nil
}

func MustEncodeOptions(v interface{}) contract.Options {
	opt, err := contract.EncodeOptions(v)
	if err != nil {
		log.Panicf("%+v", err)
	}
	return opt
}

func NewDefaultSigner(networkType, keystore, secret string) (service.Signer, error) {
	ks, err := os.ReadFile(keystore)
	if err != nil {
		return nil, err
	}
	pw, err := os.ReadFile(secret)
	if err != nil {
		return nil, err
	}
	w, err := wallet.DecryptKeyStore(ks, pw)
	if err != nil {
		return nil, err
	}
	return service.NewDefaultSigner(w, networkType), nil
}

func NewServerCommand(parentCmd *cobra.Command, parentVc *viper.Viper, version, build string, logoLines []string) (*cobra.Command, *viper.Viper) {
	rootCmd, rootVc := cli.NewCommand(parentCmd, parentVc, "server", "Server management")
	cfg := &Config{}
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cfgFilePath := rootVc.GetString("config"); cfgFilePath != "" {
			if err := ReadConfig(cfgFilePath, cfg, rootVc); err != nil {
				return err
			}
		}
		if err := rootVc.Unmarshal(&cfg, cli.ViperDecodeOptJson); err != nil {
			return fmt.Errorf("fail to unmarshall config from env err=%+v", err)
		}
		return nil
	}
	rootPFlags := rootCmd.PersistentFlags()
	rootPFlags.StringP("config", "c", "", "Parsing configuration file")
	rootPFlags.String("log_level", "debug", "Global log level (trace,debug,info,warn,error,fatal,panic)")
	rootPFlags.String("console_level", "trace", "Console log level (trace,debug,info,warn,error,fatal,panic)")
	rootPFlags.String("log_writer.filename", "btp-sdk.log", "Log file name (rotated files resides in same directory)")
	rootPFlags.Int("log_writer.maxsize", 100, "Maximum log file size in MiB")
	rootPFlags.Int("log_writer.maxage", 0, "Maximum age of log file in day")
	rootPFlags.Int("log_writer.maxbackups", 0, "Maximum number of backups")
	rootPFlags.Bool("log_writer.localtime", false, "Use localtime on rotated log file instead of UTC")
	rootPFlags.Bool("log_writer.compress", false, "Use gzip on rotated log file")
	//ServerConfig
	rootPFlags.String("server.address", "localhost:8080", "server address")
	rootPFlags.String("server.dump_log_level", "trace", "server dump log level (trace,debug,info)")
	cli.BindPFlags(rootVc, rootPFlags)

	saveCmd := &cobra.Command{
		Use:   "save [file]",
		Short: "Save configuration",
		Args:  cli.ArgsWithDefaultErrorFunc(cobra.ExactArgs(1)),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.ValidateFlagsWithViper(rootVc, cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			saveFilePath := args[0]
			cfg.FilePath, _ = filepath.Abs(saveFilePath)
			cfg.BaseDir = cfg.ResolveRelative(cfg.BaseDir)

			if cfg.LogWriter != nil {
				cfg.LogWriter.Filename = cfg.ResolveRelative(cfg.LogWriter.Filename)
			}

			if example, err := cmd.Flags().GetBool("example"); err != nil {
				return err
			} else if example {
				if len(cfg.Networks) == 0 {
					cfg.Networks = map[string]NetworkConfig{
						icon.NetworkTypeIcon + "Network": {
							NetworkType: icon.NetworkTypeIcon,
							Endpoint:    "http://localhost:9080/api/v3/icon_dex",
							Options: MustEncodeOptions(icon.AdaptorOption{
								NetworkID:         "0x3",
								TransportLogLevel: contract.LogLevel(log.TraceLevel),
							}),
							Services: map[string]contract.Options{
								bmc.ServiceName: MustEncodeOptions(service.MultiContractServiceOption{
									service.MultiContractServiceOptionNameDefault: "cx0000000000000000000000000000000000000000",
								}),
								xcall.ServiceName: MustEncodeOptions(service.DefaultServiceOptions{
									ContractAddress: "cx0000000000000000000000000000000000000000",
								}),
							},
							Signer: &SignerConfig{
								Keystore: "/path/to/keystore",
								Secret:   "/path/to/secret",
							},
						},
						eth.NetworkTypeEth2 + "Network": {
							NetworkType: eth.NetworkTypeEth2,
							Endpoint:    "http://localhost:8545",
							Options: MustEncodeOptions(eth.AdaptorOption{
								BlockMonitor: MustEncodeOptions(eth.Eth2BlockMonitorOptions{
									Endpoint: "http://localhost:9596",
								}),
								TransportLogLevel: contract.LogLevel(log.TraceLevel),
							}),
							Services: map[string]contract.Options{
								bmc.ServiceName: MustEncodeOptions(service.MultiContractServiceOption{
									service.MultiContractServiceOptionNameDefault: "0x0000000000000000000000000000000000000000",
									bmc.MultiContractServiceOptionNameBMCM:        "0x0000000000000000000000000000000000000000",
								}),
								xcall.ServiceName: MustEncodeOptions(service.DefaultServiceOptions{
									ContractAddress: "0x0000000000000000000000000000000000000000",
								}),
							},
							Signer: &SignerConfig{
								Keystore: "/path/to/keystore",
								Secret:   "/path/to/secret",
							},
						},
					}
				}
			}

			if err := cli.JsonPrettySaveFile(saveFilePath, 0644, cfg); err != nil {
				return err
			}
			cmd.Println("Save configuration to", saveFilePath)
			return nil
		},
	}
	rootCmd.AddCommand(saveCmd)
	saveCmd.Flags().Bool("example", false, "example")

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start server",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.ValidateFlagsWithViper(rootVc, cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, l := range logoLines {
				log.Println(l)
			}
			log.Printf("Version : %s", version)
			log.Printf("Build   : %s", build)

			l := log.GlobalLogger()
			if cfg.LogWriter != nil {
				var lwCfg log.WriterConfig
				lwCfg = *cfg.LogWriter
				lwCfg.Filename = cfg.ResolveAbsolute(lwCfg.Filename)
				writer, err := log.NewWriter(&lwCfg)
				if err != nil {
					log.Panicf("Fail to make writer err=%+v", err)
				}
				err = l.SetFileWriter(writer)
				if err != nil {
					log.Panicf("Fail to set file logger err=%+v", err)
				}
			}

			if lv, err := log.ParseLevel(cfg.LogLevel); err != nil {
				log.Panicf("Invalid log_level=%s", cfg.LogLevel)
			} else {
				l.SetLevel(lv)
			}
			if lv, err := log.ParseLevel(cfg.ConsoleLevel); err != nil {
				log.Panicf("Invalid console_level=%s", cfg.ConsoleLevel)
			} else {
				l.SetConsoleLevel(lv)
			}
			modLevels, _ := cmd.Flags().GetStringToString("mod_level")
			for mod, lvStr := range modLevels {
				if lv, err := log.ParseLevel(lvStr); err != nil {
					log.Panicf("Invalid mod_level mod=%s level=%s", mod, lvStr)
				} else {
					l.SetModuleLevel(mod, lv)
				}
			}
			serverDumpLogLevel, err := log.ParseLevel(cfg.Server.DumpLogLevel)
			if err != nil {
				return err
			} else {
				serverDumpLogLevel = contract.EnsureTransportLogLevel(serverDumpLogLevel)
			}
			s := api.NewServer(cfg.Server.Address, serverDumpLogLevel, l)
			svcToNetworks := make(map[string]map[string]service.Network)
			signers := make(map[string]service.Signer)
			for network, n := range cfg.Networks {
				opt, err := contract.EncodeOptions(n.Options)
				if err != nil {
					return err
				}
				a, err := contract.NewAdaptor(n.NetworkType, n.Endpoint, opt, l)
				if err != nil {
					return err
				}
				s.AddAdaptor(network, a)
				for name, so := range n.Services {
					networks, ok := svcToNetworks[name]
					if !ok {
						networks = make(map[string]service.Network)
						svcToNetworks[name] = networks
					}
					networks[network] = service.Network{
						NetworkType: n.NetworkType,
						Adaptor:     a,
						Options:     so,
					}
				}
				if n.Signer != nil {
					signer, err := NewDefaultSigner(n.NetworkType, n.Signer.Keystore, n.Signer.Secret)
					if err != nil {
						return err
					}
					signers[network] = signer
				}
			}

			for name, networks := range svcToNetworks {
				svc, err := service.NewService(name, networks, l)
				if err != nil {
					return err
				}
				if len(signers) > 0 {
					if svc, err = service.NewSignerService(svc, signers, l); err != nil {
						return err
					}
				}
				s.AddService(svc)
			}
			return s.Start()
		},
	}
	rootCmd.AddCommand(startCmd)
	startFlags := startCmd.Flags()
	startFlags.StringToString("mod_level", nil, "Set console log level for specific module ('mod'='level',...)")
	startFlags.MarkHidden("mod_level")
	return rootCmd, rootVc
}
