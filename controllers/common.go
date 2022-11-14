package controllers

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

var defaultControllerRateLimiter = workqueue.NewMaxOfRateLimiter(
	workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Minute),
	&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(1), 10)},
)
