package apiserver

// ErrorResponse error request return data
func ErrorResponse(errCode int, errMsg string) Error {
	e := Error{
		ErrorCode: errCode,
		ErrorMsg:  errMsg,
	}
	return e
}
