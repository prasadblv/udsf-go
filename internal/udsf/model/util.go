package model

import "udsf-go/internal/udsf/util"

type RequiredError = util.RequiredError
type ParsingError = util.ParsingError

const (
	errMsgMinValueConstraint = util.ErrMsgMinValueConstraint
	errMsgMaxValueConstraint = util.ErrMsgMaxValueConstraint
)

func IsZeroValue(value interface{}) bool {
	return util.IsZeroValue(value)
}
