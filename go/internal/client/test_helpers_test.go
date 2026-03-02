package client

import "runtime"

func windowsRuleBonus() int {
	if runtime.GOOS == "windows" {
		return 2
	}
	return 0
}
