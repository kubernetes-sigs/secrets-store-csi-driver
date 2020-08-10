package framework

import (
	"time"
)

const (
	CreateTimeout = 10 * time.Second
	CreatePolling = 1 * time.Second

	DeleteTimeout = 10 * time.Second
	DeletePolling = 1 * time.Second

	ListTimeout = 10 * time.Second
	ListPolling = 1 * time.Second

	GetTimeout = 1 * time.Minute
	GetPolling = 5 * time.Second

	UpdateTimeout = 10 * time.Second
	UpdatePolling = 1 * time.Second

	WaitTimeout = 5 * time.Minute
	WaitPolling = 5 * time.Second

	HelmTimeout = time.Duration(15) * time.Minute
)
