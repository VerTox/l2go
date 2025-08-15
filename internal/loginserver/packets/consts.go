package packets

const (
	REASON_SYSTEM_ERROR_LOGIN_LATER      = 0x01
	REASON__PASS_WRONG                   = 0x02
	REASON_USER_OR_PASS_WRONG            = 0x03
	REASON_ACCESS_FAILED_TRY_AGAIN_LATER = 0x04
	REASON_INFO_WRONG                    = 0x05
	REASON_ACCOUNT_IN_USE                = 0x07
	REASON_MAINTENANCE                   = 0x10
	REASON_CHANGE_TMP_PASS               = 0x11
	REASON_EXPIRED                       = 0x12
	REASON_NO_TIME_LEFT                  = 0x13
	REASON_SYSTEM_ERROR2                 = 0x14
	REASON_ACCESS_FAILED                 = 0x15
	REASON_ACCOUNT_SUSPENDED_CALL        = 0x28
)
