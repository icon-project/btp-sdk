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

package icon

import (
	"encoding/json"

	"github.com/icon-project/btp2/common/errors"

	"github.com/icon-project/btp-sdk/contract"
)

func init() {
	contract.RegisterSpecFactory(NewSpec, NetworkTypes...)
}

func NewSpec(b []byte) (*contract.Spec, error) {
	spec := &contract.Spec{}
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, errors.Wrapf(err, "fail to unmarshal spec err:%s", err.Error())
	}
	return spec, nil
}
