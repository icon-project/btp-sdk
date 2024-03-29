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
	"github.com/icon-project/btp2/common/intconv"
	"github.com/icon-project/btp2/common/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/icon-project/btp-sdk/api"
	"github.com/icon-project/btp-sdk/contract"
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

func NewServicesCommand(parentCmd *cobra.Command, parentVc *viper.Viper) (*cobra.Command, *viper.Viper) {
	rootCmd, rootVc := cli.NewCommand(parentCmd, parentVc, "services", "Get list of service information")
	var (
		c api.Client
	)
	rootCmd.PersistentPreRunE = ClientPersistentPreRunE(rootVc, &c)
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		r, err := c.ServiceInfos()
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

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "[Experimental] Register contract service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := os.ReadFile(cmd.Flag("contract.spec").Value.String())
			if err != nil {
				return err
			}
			registerReq := &api.RegisterContractServiceRequest{
				Network: network,
				Address: contract.Address(cmd.Flag("contract.address").Value.String()),
				Spec:    spec,
			}
			if err = c.RegisterContractService(registerReq); err != nil {
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

	finality := &cobra.Command{
		Use:   "finality BLOCK_ID",
		Short: "GetFinality",
		Args:  cli.ArgsWithDefaultErrorFunc(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := intconv.ParseInt(cmd.Flag("height").Value.String(), 64)
			if err != nil {
				return err
			}
			ret, err := c.GetFinality(network, args[0], height)
			if err != nil {
				return err
			}
			return cli.JsonPrettyPrintln(os.Stdout, ret)
		},
	}
	finality.Flags().String("height", "0", "height")
	rootCmd.AddCommand(finality)

	methodInfosCmd := &cobra.Command{
		Use:   "methods",
		Short: "[Experimental] Get list of method information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := cmd.Flag("service").Value.String()
			txr, err := c.MethodInfos(network, svc)
			if err != nil {
				return err
			}
			return cli.JsonPrettyPrintln(os.Stdout, txr)
		},
	}
	rootCmd.AddCommand(methodInfosCmd)
	methodInfosFlags := methodInfosCmd.Flags()
	methodInfosFlags.String("service", "", "service name")
	cli.MarkAnnotationRequired(methodInfosFlags, "service")

	var (
		method string
		req    = &api.Request{}
	)
	newMethodApiCommand := func(use, short string) *cobra.Command {
		cmd := &cobra.Command{
			Use:   fmt.Sprintf("%s METHOD", use),
			Short: short,
			Args:  cli.ArgsWithDefaultErrorFunc(cobra.ExactArgs(1)),
			PreRunE: func(cmd *cobra.Command, args []string) error {
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
				req.Network = network
				if req.Params, err = GetStringToInterface(fs, "param"); err != nil {
					return err
				}
				if req.Options, err = GetStringToInterface(fs, "option"); err != nil {
					return err
				}
				return nil
			},
		}
		fs := cmd.Flags()
		fs.String("service", "", "service name")
		fs.StringToString("param", nil,
			"key=value, Function parameters, if '--raw' used, will overwrite")
		fs.StringToString("option", nil,
			"key=value, Call options, if '--raw' used, will overwrite")
		fs.String("raw", "", "call with 'data' using raw json file or json-string")
		//fs.StringToString("params", nil,
		//	"raw json string or '@<json file>' or '-' for stdin for parameter JSON. it overrides raw one ")
		cli.MarkAnnotationRequired(fs, "service")
		return cmd
	}

	callCmd := newMethodApiCommand("call", "Call")
	callCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var resp interface{}
		svc := cmd.Flag("service").Value.String()
		if _, err := c.Call(svc, method, req, &resp); err != nil {
			return err
		}
		if err := cli.JsonPrettyPrintln(os.Stdout, resp); err != nil {
			return errors.Errorf("failed JsonIntend resp=%+v, err=%+v", resp, err)
		}
		return nil
	}
	rootCmd.AddCommand(callCmd)

	invokeCmd := newMethodApiCommand("invoke", "Invoke")
	invokeCmd.RunE = func(cmd *cobra.Command, args []string) error {
		keystore := cmd.Flag("keystore").Value.String()
		secret := cmd.Flag("secret").Value.String()
		s, err := NewDefaultSigner(networkType, keystore, secret)
		if err != nil {
			return err
		}
		var txID contract.TxID
		svc := cmd.Flag("service").Value.String()
		txID, err = c.Invoke(svc, method, req, s)
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
