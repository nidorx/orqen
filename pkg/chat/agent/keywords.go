package agent

// Confirmation Keywords

var approvalKeywords = map[string]bool{
	"yes": true, "y": true, "ok": true, "approve": true,
	"apply": true, "do it": true, "go ahead": true,
}

var rejectionKeywords = map[string]bool{
	"no": true, "n": true, "cancel": true, "reject": true,
	"discard": true, "skip": true, "dont": true, "don't": true,
}

func isApprovalKeyword(text string) bool {
	return approvalKeywords[text]
}

func isRejectionKeyword(text string) bool {
	return rejectionKeywords[text]
}
