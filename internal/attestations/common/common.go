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

// UnmarshalPBStruct converts a *structpb.Struct into a Go struct.
func UnmarshalPBStruct(pbStruct *structpb.Struct, out any) error {
	// Convert structpb.Struct to JSON bytes
	jsonBytes, err := pbStruct.MarshalJSON()
	if err != nil {
		return err
	}

	// Unmarshal into the provided Go struct
	return json.Unmarshal(jsonBytes, out)
}
