// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"os"
)

const EvalModeKey = "GITTUF_EVAL"

var ErrNotInEvalMode = fmt.Errorf("this feature is only available with eval mode, and can UNDERMINE repository security; override by setting %s=1", EvalModeKey)

// InEvalMode returns true if gittuf is currently in eval mode.
func InEvalMode() bool {
	return os.Getenv(EvalModeKey) == "1"
}
