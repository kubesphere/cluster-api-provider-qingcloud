package qingcloud

import (
	"errors"
	"regexp"

	qcerrors "github.com/yunify/qingcloud-sdk-go/request/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

const (
	CodeInternalServerError    = 5000
	CodeResourceAlreadyExisted = 2110
	CodeResourceNotFound       = 2100
)

func NewQingCloudError(retCode *int, message *string) *qcerrors.QingCloudError {
	return &qcerrors.QingCloudError{
		RetCode: qcs.IntValue(retCode),
		Message: qcs.StringValue(message),
	}
}

func IsQingCloudError(err error) (*qcerrors.QingCloudError, bool) {
	if err == nil {
		return nil, false
	}
	qcErr := &qcerrors.QingCloudError{}
	ok := errors.As(err, qcErr)
	if ok {
		return qcErr, true
	}
	return nil, ok
}

func isAlreadyExistedErrorMessage(message string) bool {
	pat := "\\w+ already existed"
	reg := regexp.MustCompile(pat)
	return reg.MatchString(message)
}

func IsAlreadyExisted(err error) bool {
	if err == nil {
		return false
	}
	qcErr, ok := IsQingCloudError(err)
	if !ok {
		return false
	}
	return qcErr.RetCode == CodeResourceAlreadyExisted || (qcErr.RetCode == CodeInternalServerError && isAlreadyExistedErrorMessage(qcErr.Message))
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	qcErr, ok := IsQingCloudError(err)
	if !ok {
		return false
	}
	return qcErr.RetCode == CodeResourceNotFound
}
