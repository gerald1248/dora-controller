package main

func getDeploymentFlags(success bool, previousSuccess bool, imageChanged bool) string {
	var s string
	if success && imageChanged {
		s += "DORA_SUCCESS"
	} else if success && !previousSuccess {
		s += "DORA_RECOVERY"
	} else if success {
		s += "DORA_REDEPLOY"
	} else {
		s += "DORA_FAILURE"
	}

	if imageChanged {
		s += "|DORA_NEW_IMAGE"
	} else {
		s += "|DORA_SAME_IMAGE"
	}

	if previousSuccess {
		s += "|DORA_PREVIOUS_SUCCESS"
	} else {
		s += "|DORA_PREVIOUS_FAILURE"
	}

	return s
}
