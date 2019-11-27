/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	secretsstore "github.com/deislabs/secrets-store-csi-driver/pkg/secrets-store"
)

var (
	endpoint           = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName         = flag.String("drivername", "secrets-store.csi.k8s.com", "name of the driver")
	nodeID             = flag.String("nodeid", "", "node id")
	debug              = flag.Bool("debug", false, "sets log to debug level")
	logFormatJSON      = flag.Bool("log-format-json", false, "set log formatter to json")
	logReportCaller    = flag.Bool("log-report-caller", false, "include the calling method as fields in the log")
	providerVolumePath = flag.String("provider-volume", "/etc/kubernetes/secrets-store-csi-providers", "Volume path for provider")
)

func main() {
	flag.Parse()

	log.SetLevel(log.InfoLevel)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	if *logFormatJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}

	log.SetReportCaller(*logReportCaller)

	handle()
}

func handle() {
	driver := secretsstore.GetDriver()
	driver.Run(*driverName, *nodeID, *endpoint, *providerVolumePath)
}
