/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/icon-project/btp2/common/cli"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/icon-project/btp-sdk/api"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

func GetStringToInterface(fs *pflag.FlagSet, name string) (map[string]interface{}, error) {
	m, err := fs.GetStringToString(name)
	if err != nil {
		return nil, err
	}
	r := make(map[string]interface{})
	for k, v := range m {
		r[k] = v
	}
	return r, nil
}

func ReadAndUnmarshal(file string, v interface{}) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func ClientPersistentPreRunE(vc *viper.Viper, c *api.Client) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := cli.ValidateFlagsWithViper(vc, cmd.Flags()); err != nil {
			return err
		}
		l := log.GlobalLogger()
		if lv, err := log.ParseLevel(vc.GetString("log_level")); err != nil {
			return errors.Wrapf(err, "fail to parseLevel log_level err:%s", err.Error())
		} else {
			l.SetLevel(lv)
		}
		if lv, err := log.ParseLevel(vc.GetString("console_level")); err != nil {
			return errors.Wrapf(err, "fail to parseLevel console_level err:%s", err.Error())
		} else {
			l.SetConsoleLevel(lv)
		}
		dumpLogLevel, err := log.ParseLevel(vc.GetString("dump_log_level"))
		if err != nil {
			return errors.Wrapf(err, "fail to parseLevel dump_log_level err:%s", err.Error())
		} else {
			dumpLogLevel = contract.EnsureTransportLogLevel(dumpLogLevel)
		}
		networkToType := make(map[string]string)
		if network := vc.GetString("network.name"); len(network) > 0 {
			networkToType[network] = vc.GetString("network.type")
		}
		*c = *api.NewClient(
			vc.GetString("url"),
			networkToType,
			dumpLogLevel,
			l)
		return nil
	}
}

func AddAdminRequiredFlags(c *cobra.Command) {
	pFlags := c.PersistentFlags()
	pFlags.String("url", "http://localhost:8080", "server address")
	pFlags.String("log_level", "debug", "Global log level (trace,debug,info,warn,error,fatal,panic)")
	pFlags.String("console_level", "trace", "Console log level (trace,debug,info,warn,error,fatal,panic)")
	pFlags.String("dump_log_level", "trace", "client dump log level (trace,debug,info)")
	pFlags.String("network.name", "", "network name")
	pFlags.String("network.type", "", "network type")
}

func NewNetworksCommand(parentCmd *cobra.Command, parentVc *viper.Viper) (*cobra.Command, *viper.Viper) {
	rootCmd, rootVc := cli.NewCommand(parentCmd, parentVc, "networks", "Get list of network information")
	var (
		c api.Client
	)
	rootCmd.PersistentPreRunE = ClientPersistentPreRunE(rootVc, &c)
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		r, err := c.NetworkInfos()
		if err != nil {
			return err
		}
		return cli.JsonPrettyPrintln(os.Stdout, r)
	}
	AddAdminRequiredFlags(rootCmd)
	cli.BindPFlags(rootVc, rootCmd.PersistentFlags())
	return rootCmd, rootVc
}

func NewApiCommand(parentCmd *cobra.Command, parentVc *viper.Viper) (*cobra.Command, *viper.Viper) {
	rootCmd, rootVc := cli.NewCommand(parentCmd, parentVc, "api", "API cli")
	var (
		c           api.Client
		network     string
		networkType string
	)
	persistentPreRunE := ClientPersistentPreRunE(rootVc, &c)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := persistentPreRunE(cmd, args); err != nil {
			return err
		}
		network = rootVc.GetString("network.name")
		networkType = rootVc.GetString("network.type")
		return nil
	}
	AddAdminRequiredFlags(rootCmd)
	cli.MarkAnnotationCustom(rootCmd.PersistentFlags(), "network.name", "network.type")
	cli.BindPFlags(rootVc, rootCmd.PersistentFlags())

	rootCmd.AddCommand(&cobra.Command{
		Use:   "services",
		Short: "Get list of service information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := c.ServiceInfos(network)
			if err != nil {
				return err
			}
			return cli.JsonPrettyPrintln(os.Stdout, r)
		},
	})
	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register contract service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := os.ReadFile(cmd.Flag("contract.spec").Value.String())
			if err != nil {
				return err
			}
			registerReq := &api.RegisterContractServiceRequest{
				Address: contract.Address(cmd.Flag("contract.address").Value.String()),
				Spec:    spec,
			}
			if err = c.RegisterContractService(network, registerReq); err != nil {
				return err
			}
			cmd.Println("Operation success")
			return nil
		},
	}
	rootCmd.AddCommand(registerCmd)
	registerFlags := registerCmd.Flags()
	registerFlags.String("contract.address", "", "contract address")
	registerFlags.String("contract.spec", "", "contract spec")
	cli.MarkAnnotationRequired(registerFlags, "contract.address", "contract.spec")

	resultCmd := &cobra.Command{
		Use:   "result TX_ID",
		Short: "GetResult",
		Args:  cli.ArgsWithDefaultErrorFunc(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			txr, err := c.GetResult(network, args[0])
			if err != nil {
				return err
			}
			return cli.JsonPrettyPrintln(os.Stdout, txr)
		},
	}
	rootCmd.AddCommand(resultCmd)

	var (
		svc  string
		addr contract.Address
	)
	serviceApiPreRunE := func(cmd *cobra.Command, args []string) error {
		svc = cmd.Flag("service").Value.String()
		addr = contract.Address(cmd.Flag("contract.address").Value.String())
		if len(svc) == 0 && len(addr) == 0 {
			return errors.New("require service or contract.address")
		}
		return nil
	}
	methodInfosCmd := &cobra.Command{
		Use:     "methods",
		Short:   "Get list of method information",
		Args:    cobra.NoArgs,
		PreRunE: serviceApiPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(svc) == 0 {
				svc = string(addr)
			}
			txr, err := c.MethodInfos(network, svc)
			if err != nil {
				return err
			}
			return cli.JsonPrettyPrintln(os.Stdout, txr)
		},
	}
	methodInfosCmd.Flags().String("service", "", "service name")
	methodInfosCmd.Flags().String("contract.address", "", "contract address, if '--service' used, will be ignored")
	rootCmd.AddCommand(methodInfosCmd)

	var (
		method string
		req    = &api.ContractRequest{}
	)
	newMethodApiCommand := func(use, short string) *cobra.Command {
		cmd := &cobra.Command{
			Use:   fmt.Sprintf("%s METHOD", use),
			Short: short,
			Args:  cli.ArgsWithDefaultErrorFunc(cobra.ExactArgs(1)),
			PreRunE: func(cmd *cobra.Command, args []string) error {
				if err := serviceApiPreRunE(cmd, args); err != nil {
					return err
				}
				method = args[0]
				var (
					fs  = cmd.Flags()
					err error
				)
				if raw := cmd.Flag("raw").Value.String(); len(raw) > 0 {
					if err = ReadAndUnmarshal(raw, req); err != nil {
						return err
					}
				}
				if req.Params, err = GetStringToInterface(fs, "param"); err != nil {
					return err
				}
				if req.Options, err = GetStringToInterface(fs, "option"); err != nil {
					return err
				}
				if specFile := cmd.Flag("contract.spec").Value.String(); len(svc) == 0 && len(addr) > 0 && len(specFile) > 0 {
					if req.Spec, err = os.ReadFile(specFile); err != nil {
						return err
					}
				}
				return nil
			},
		}
		fs := cmd.Flags()
		fs.String("service", "", "service name")
		fs.String("contract.address", "", "contract address, if '--service' used, will be ignored")
		fs.String("contract.spec", "", "contract spec, if '--service' used, will be ignored")
		fs.String("method", "",
			"Name of the function to invoke, if '--raw' used, will overwrite")
		fs.StringToString("param", nil,
			"key=value, Function parameters, if '--raw' used, will overwrite")
		fs.StringToString("option", nil,
			"key=value, Call options, if '--raw' used, will overwrite")
		fs.String("raw", "", "call with 'data' using raw json file or json-string")
		//fs.StringToString("params", nil,
		//	"raw json string or '@<json file>' or '-' for stdin for parameter JSON. it overrides raw one ")
		return cmd
	}

	callCmd := newMethodApiCommand("call", "Call")
	callCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var (
			err  error
			resp interface{}
		)
		if len(svc) > 0 {
			_, err = c.ServiceCall(network, svc, method, &req.Request, &resp)
		} else if len(addr) > 0 {
			_, err = c.Call(network, addr, method, req, &resp)
		}
		if err != nil {
			return err
		}
		if err = cli.JsonPrettyPrintln(os.Stdout, resp); err != nil {
			return errors.Errorf("failed JsonIntend resp=%+v, err=%+v", resp, err)
		}
		return nil
	}
	rootCmd.AddCommand(callCmd)

	invokeCmd := newMethodApiCommand("invoke", "Invoke")
	invokeCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var (
			err  error
			txID contract.TxID
			s    service.Signer
		)
		keystore := cmd.Flag("keystore").Value.String()
		secret := cmd.Flag("secret").Value.String()
		if s, err = NewDefaultSigner(networkType, keystore, secret); err != nil {
			return err
		}
		if len(svc) > 0 {
			txID, err = c.ServiceInvoke(network, svc, method, &req.Request, s)
		} else if len(addr) > 0 {
			txID, err = c.Invoke(network, addr, method, req, s)
		}
		if err != nil {
			return err
		}
		if err = cli.JsonPrettyPrintln(os.Stdout, txID); err != nil {
			return errors.Errorf("failed JsonIntend resp=%+v, err=%+v", txID, err)
		}
		return nil
	}
	rootCmd.AddCommand(invokeCmd)
	invokeFlags := invokeCmd.Flags()
	invokeFlags.String("keystore", "", "keystore file path")
	invokeFlags.String("secret", "", "secret file path")
	cli.MarkAnnotationRequired(invokeFlags, "keystore", "secret")
	return rootCmd, rootVc
}
