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

func NewApiCommand(parentCmd *cobra.Command, parentVc *viper.Viper) (*cobra.Command, *viper.Viper) {
	rootCmd, rootVc := cli.NewCommand(parentCmd, parentVc, "api", "API cli")
	var (
		c           *api.Client
		network     string
		networkType string
		svc         string
		addr        contract.Address
		req         = &api.ContractRequest{}
	)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := cli.ValidateFlagsWithViper(rootVc, cmd.Flags()); err != nil {
			return err
		}
		l := log.GlobalLogger()
		if lv, err := log.ParseLevel(rootVc.GetString("log_level")); err != nil {
			return errors.Wrapf(err, "fail to parseLevel log_level err:%s", err.Error())
		} else {
			l.SetLevel(lv)
		}
		if lv, err := log.ParseLevel(rootVc.GetString("console_level")); err != nil {
			return errors.Wrapf(err, "fail to parseLevel console_level err:%s", err.Error())
		} else {
			l.SetConsoleLevel(lv)
		}
		dumpLogLevel, err := log.ParseLevel(rootVc.GetString("dump_log_level"))
		if err != nil {
			return errors.Wrapf(err, "fail to parseLevel dump_log_level err:%s", err.Error())
		} else {
			dumpLogLevel = contract.EnsureTransportLogLevel(dumpLogLevel)
		}
		network = rootVc.GetString("network.name")
		networkType = rootVc.GetString("network.type")
		c = api.NewClient(
			rootVc.GetString("url"),
			map[string]string{network: networkType},
			dumpLogLevel,
			l)
		return nil
	}

	rootPFlags := rootCmd.PersistentFlags()
	rootPFlags.String("url", "http://localhost:8080", "server address")
	rootPFlags.String("log_level", "debug", "Global log level (trace,debug,info,warn,error,fatal,panic)")
	rootPFlags.String("console_level", "trace", "Console log level (trace,debug,info,warn,error,fatal,panic)")
	rootPFlags.String("dump_log_level", "trace", "client dump log level (trace,debug,info)")
	rootPFlags.String("network.name", "", "network name")
	rootPFlags.String("network.type", "", "network type")
	cli.MarkAnnotationCustom(rootPFlags, "server.address", "network.name", "network.type", "method")
	cli.BindPFlags(rootVc, rootPFlags)

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

	newServiceApiCommand := func(use, short string) *cobra.Command {
		cmd := &cobra.Command{
			Use:   use,
			Short: short,
			Args:  cobra.NoArgs,
			PreRunE: func(cmd *cobra.Command, args []string) error {
				svc = cmd.Flag("service").Value.String()
				addr = contract.Address(cmd.Flag("contract.address").Value.String())
				if len(svc) == 0 && len(addr) == 0 {
					return errors.New("require service or contract.address")
				}
				var (
					fs  = cmd.Flags()
					err error
				)
				if raw := cmd.Flag("raw").Value.String(); len(raw) > 0 {
					if err = ReadAndUnmarshal(raw, req); err != nil {
						return err
					}
				}
				if req.Method, err = fs.GetString("method"); err != nil {
					return err
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
		fs.String("contract.address", "", "contract address")
		fs.String("contract.spec", "", "contract spec")
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

	callCmd := newServiceApiCommand("call", "Call")
	callCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var (
			err  error
			resp interface{}
		)
		if len(svc) > 0 {
			_, err = c.ServiceCall(network, svc, &req.Request, &resp)
		} else if len(addr) > 0 {
			_, err = c.Call(network, addr, req, &resp)
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

	invokeCmd := newServiceApiCommand("invoke", "Invoke")
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
			txID, err = c.ServiceInvoke(network, svc, &req.Request, s)
		} else if len(addr) > 0 {
			txID, err = c.Invoke(network, addr, req, s)
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
