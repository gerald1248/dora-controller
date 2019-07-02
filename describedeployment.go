package main

import (
	"fmt"
	au "github.com/logrusorgru/aurora"
	"strings"
)

func describeDeployment(deployment Deployment) string {
	var items []string
	items = append(items, fmt.Sprintf("%s:", au.Bold(au.Cyan("DEBUG"))))
	items = append(items, fmt.Sprintf("Name=%s", au.Bold(deployment.Name)))
	items = append(items, fmt.Sprintf(" Namespace=%s", au.Bold(deployment.Namespace)))
	items = append(items, fmt.Sprintf(" Image=%s", au.Bold(deployment.Image)))
	items = append(items, fmt.Sprintf(" ImageChanged=%t", au.Bold(deployment.ImageChanged)))
	items = append(items, fmt.Sprintf(" Flags=%s", au.Bold(deployment.Flags)))
	items = append(items, fmt.Sprintf(" LastTimestamp=%d", au.Bold(deployment.LastTimestamp)))
	items = append(items, fmt.Sprintf(" RecoverySeconds=%d", au.Bold(deployment.RecoverySeconds)))
	if deployment.CommitTimestamp > 0 {
		items = append(items, fmt.Sprintf(" CommitTimestamp=%d", au.Bold(deployment.CommitTimestamp)))
		items = append(items, fmt.Sprintf(" CycleTimeSeconds=%d", au.Bold(deployment.CycleTimeSeconds)))
	}
	items = append(items, fmt.Sprintf(" Success=%t", au.Bold(deployment.Success)))
	return strings.Join(items, " ")
}
