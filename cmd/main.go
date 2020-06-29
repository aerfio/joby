package main

import (
	logf "github.com/sirupsen/logrus"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	if err := mainerr(); err != nil {
		logf.Fatal(err)
	}
	logf.Info("success!")
}
