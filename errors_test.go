package xlog

import "errors"
import "testing"

func TestBatchResult(t *testing.T) {
	r := BatchResult{}
	if r.Errors() != nil {
		t.Errorf(".Errors() returns non-nil value although there are no errors")
	}
	if r.Error() != "successful, no errors" {
		t.Errorf(".Error() returns wrong message in case of no errors")
	}

	r.Fail("rec1", errors.New("some error 1"))
	if len(r.Errors()) != 1 {
		t.Errorf(".Fial() does not add an error")
	}
	r.OK("rec1")
	if len(r.ListOfSuccessful()) != 1 {
		t.Errorf(".OK does not add items to list\n%v", r.successful)
	}
	if r.Errors() != nil { // check list drop
		t.Errorf(".Errors() returns non-nil value after reset")
	}

	r.Fail("rec1", errors.New("some error 1"))
	if len(r.ListOfSuccessful()) != 0 { // check list drop
		t.Errorf(".ListOfsuccessful() returns value after reset")
	}
	r.Fail("rec2", errors.New("some error 2"))

	const errTail = " (rec1, rec2)"
	const errNoMsg = "unknown errors"
	if msg := r.Error(); msg != errNoMsg + errTail {
		t.Errorf("wrong .Error() return\nreturn: %s\nshould be: %s",
			msg, errNoMsg + errTail)
	}
	const errMsg = "an error class"
	r.SetMsg(errMsg)
	if msg := r.Error(); msg != errMsg + errTail {
		t.Errorf("wrong .Error() return\nreturn: %s\nshould be: %s",
			msg, errMsg + errTail)
	}
}
