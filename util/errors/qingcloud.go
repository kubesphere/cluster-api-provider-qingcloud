package errors

import (
	"regexp"

	"github.com/pkg/errors"
	qcerrors "github.com/yunify/qingcloud-sdk-go/request/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

const (
	QingCloudInternalServerErrorCode = 5000
)

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

func isQingCloudAlreadyExistedErrorMessage(message string) bool {
	pat := "\\w+ already existed\\."
	reg := regexp.MustCompile(pat)
	return reg.MatchString(message)
}

func IsQingCloudResourceAlreadyExistedError(err error) bool {
	if err == nil {
		return false
	}
	qcErr, ok := IsQingCloudError(err)
	if !ok {
		return false
	}
	return qcErr.RetCode == QingCloudInternalServerErrorCode && isQingCloudAlreadyExistedErrorMessage(qcErr.Message)
}

func NewQingCloudError(retCode *int, message *string) *qcerrors.QingCloudError {
	return &qcerrors.QingCloudError{
		RetCode: qcs.IntValue(retCode),
		Message: qcs.StringValue(message),
	}
}
