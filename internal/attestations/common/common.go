// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"

	"google.golang.org/protobuf/types/known/structpb"
)

func PredicateToPBStruct(predicate any) (*structpb.Struct, error) {
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
