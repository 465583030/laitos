package feature

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// An undocumented way to send message to myself.
type Undocumented1 struct {
	URL   string
	Addr1 string
	Addr2 string
	ID1   string
	ID2   string
}

var TestUndocumented1 = Undocumented1{} // Details are set by init_test.go

func (und *Undocumented1) IsConfigured() bool {
	return und.URL != "" && und.Addr1 != "" && und.Addr2 != "" && und.ID1 != "" && und.ID2 != ""
}

func (und *Undocumented1) SelfTest() error {
	if !und.IsConfigured() {
		return ErrIncompleteConfig
	}
	status, _, err := DoHTTP(HTTP_TEST_TIMEOUT_SEC, "GET", "", nil, nil, und.URL)
	// Only consider 404 to be an actual error
	if status == 404 {
		return err
	}
	return nil
}

func (und *Undocumented1) Initialise() error {
	return nil
}

func (und *Undocumented1) TriggerPrefix() string {
	return "NOT-TO-BE-TRIGGERED-MANUALLY"
}

func (und *Undocumented1) Execute(cmd Command) (ret *Result) {
	LogBeforeExecute(cmd)
	defer func() {
		LogAfterExecute(cmd, ret)
	}()

	if errResult := cmd.Trim(); errResult != nil {
		ret = errResult
		return
	}

	params := url.Values{"ReplyAddress": {und.Addr2}, "ReplyMessage": {cmd.Content}, "MessageId": {und.ID1}, "Guid": {und.ID2}}
	status, resp, err := DoHTTP(cmd.TimeoutSec, "POST", "", strings.NewReader(params.Encode()), func(req *http.Request) error {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36")
		return nil
	}, und.URL)

	if errResult := HTTPResponseError(status, resp, err); errResult != nil {
		ret = errResult
	} else {
		// The OK output is simply the length message
		ret = &Result{Error: nil, Output: strconv.Itoa(len(cmd.Content))}
	}
	return
}