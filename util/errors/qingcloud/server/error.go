package server

import (
	"regexp"

	"github.com/kubesphere/cluster-api-provider-qingcloud/util/errors/qingcloud"
)

const (
	QingCloudInternalServerErrorCode = 5000
)

func isAlreadyExistedErrorMessage(message string) bool {
	pat := "\\w+ already existed"
	reg := regexp.MustCompile(pat)
	return reg.MatchString(message)
}

func IsAlreadyExisted(err error) bool {
	if err == nil {
		return false
	}
	qcErr, ok := qingcloud.IsQingCloudError(err)
	if !ok {
		return false
	}
	return qcErr.RetCode == QingCloudInternalServerErrorCode && isAlreadyExistedErrorMessage(qcErr.Message)
}
