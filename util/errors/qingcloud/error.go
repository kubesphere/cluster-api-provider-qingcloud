package qingcloud

import (
	"errors"

	qcerrors "github.com/yunify/qingcloud-sdk-go/request/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
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
