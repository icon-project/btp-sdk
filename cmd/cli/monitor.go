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
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/icon-project/btp2/common/cli"
	"github.com/icon-project/btp2/common/errors"
	"github.com/icon-project/btp2/common/intconv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/icon-project/btp-sdk/api"
	"github.com/icon-project/btp-sdk/contract"
)

func NewMonitorCommand(parentCmd *cobra.Command, parentVc *viper.Viper) (*cobra.Command, *viper.Viper) {
	rootCmd, rootVc := cli.NewCommand(parentCmd, parentVc, "monitor", "Monitor cli")
	var (
		c       api.Client
		network string
	)
	persistentPreRunE := ClientPersistentPreRunE(rootVc, &c)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := persistentPreRunE(cmd, args); err != nil {
			return err
		}
		network = rootVc.GetString("network.name")
		return nil
	}
	AddAdminRequiredFlags(rootCmd)
	cli.MarkAnnotationCustom(rootCmd.PersistentFlags(), "network.name", "network.type")
	cli.BindPFlags(rootVc, rootCmd.PersistentFlags())

	eventCmd := &cobra.Command{
		Use:   "event",
		Short: "Event monitor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			height, err := intconv.ParseInt(cmd.Flag("height").Value.String(), 64)
			if err != nil {
				return err
			}
			req := &api.EventMonitorRequest{
				Height:       height,
				NameToParams: make(map[string][]contract.Params),
			}
			if raw := cmd.Flag("raw").Value.String(); len(raw) > 0 {
				if err = ReadAndUnmarshal(raw, req); err != nil {
					return err
				}
			}
			nameToRawJson, err := cmd.Flags().GetStringToString("filter")
			if err != nil {
				return err
			}
			if len(nameToRawJson) == 0 {
				return errors.New("require filter at least one")
			}
			for name, rawJson := range nameToRawJson {
				var params contract.Params
				if len(rawJson) > 0 {
					var b []byte
					if strings.HasPrefix(strings.TrimSpace(rawJson), "{") {
						b = []byte(rawJson)
					} else {
						if b, err = os.ReadFile(rawJson); err != nil {
							return err
						}
					}
					if err = json.Unmarshal(b, &params); err != nil {
						return err
					}
				}
				l, ok := req.NameToParams[name]
				if !ok {
					l = make([]contract.Params, 0)
				}
				req.NameToParams[name] = append(l, params)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cli.OnInterrupt(cancel)
			onEvent := func(e contract.Event) error {
				return cli.JsonPrettyPrintln(os.Stdout, e)
			}
			svc := cmd.Flag("service").Value.String()
			return c.MonitorEvent(ctx, network, svc, req, onEvent)
		},
	}
	rootCmd.AddCommand(eventCmd)
	eventFlags := eventCmd.Flags()
	eventFlags.String("service", "", "service name")
	eventFlags.StringToString("filter", nil,
		"Event=Filter, raw json file or json string, if '--raw' used, will overwrite")
	eventFlags.String("height", "0", "height, if '--raw' used, will overwrite")
	eventFlags.String("raw", "", "call with 'data' using raw json file or json-string")
	cli.MarkAnnotationRequired(eventFlags, "service")
	return rootCmd, rootVc
}
