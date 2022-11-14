package qingcloud

import (
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
	qcErr, ok := err.(*qcerrors.QingCloudError)
	if ok {
		return qcErr, true
	}
	return nil, ok
}

func isAlreadyExistedErrorMessage(message string) bool {
	pat := ".+ already existed"
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

func isDeletedErrorMessage(message string) bool {
	pat := ".+ has already been deleted"
	reg := regexp.MustCompile(pat)
	return reg.MatchString(message)
}

func IsDeleted(err error) bool {
	if err == nil {
		return false
	}
	qcErr, ok := IsQingCloudError(err)
	if !ok {
		return false
	}
	return isDeletedErrorMessage(qcErr.Message)
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

func IsNotFoundOrDeleted(err error) bool {
	return IsNotFound(err) || IsDeleted(err)
}
