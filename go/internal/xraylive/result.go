package xraylive

import "fmt"

func ResultError(result RuntimeApplyResult) error {
	switch result {
	case RuntimeApplyApplied, RuntimeApplyNoop, RuntimeApplyStaged:
		return nil
	case RuntimeApplyFailed:
		return fmt.Errorf("runtime apply failed")
	case RuntimeApplyUnsupported:
		return fmt.Errorf("runtime apply unsupported")
	case RuntimeApplyServiceLayerRequired:
		return fmt.Errorf("runtime apply requires service layer")
	case RuntimeApplySkipped:
		return fmt.Errorf("runtime apply skipped")
	default:
		return fmt.Errorf("runtime apply returned %s", result)
	}
}
