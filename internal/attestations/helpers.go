// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"

	"google.golang.org/protobuf/types/known/structpb"
)

func predicateToPBStruct(predicate any) (*structpb.Struct, error) {
	predicateBytes, err := json.Marshal(predicate)
	if err != nil {
		return nil, err
	}

	predicateInterface := &map[string]any{}
	if err := json.Unmarshal(predicateBytes, predicateInterface); err != nil {
		return nil, err
	}

	return structpb.NewStruct(*predicateInterface)
}
