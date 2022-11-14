package controllers

import (
	"time"

	"k8s.io/client-go/util/workqueue"
)

var defaultControllerRateLimiter = workqueue.NewItemFastSlowRateLimiter(10*time.Second, 10*time.Minute, 600)
