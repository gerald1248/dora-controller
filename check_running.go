package main

import (
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	au "github.com/logrusorgru/aurora"
)

func checkRunning(clientset kubernetes.Interface, name string, namespace string) bool {
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		log(fmt.Sprintf("%s", au.Bold(au.Red("can't fetch pods"))))
		return false
	}
	if len(pods.Items) == 0 {
		log(fmt.Sprintf("%s", au.Bold(au.Red("no pods available"))))
		return false
	}
	for _, pod := range pods.Items {
		podName := pod.ObjectMeta.Name
		if !strings.HasPrefix(podName, name) {
			log(fmt.Sprintf("ignoring pod %s", podName))
			continue
		}
		phase := pod.Status.Phase
		if phase != "Running" {
			log(fmt.Sprintf("pod %s not in phase Running", podName))
			return false
		}
	}
	return true
}
