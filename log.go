package main

import (
	"fmt"
	"os"
	"time"
        au "github.com/logrusorgru/aurora"
)

func log(s string) {
	time := time.Now()
	timestamp := fmt.Sprintf("%s", time.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(os.Stderr, "[%s] %s\n", timestamp, au.Bold(s))
}
