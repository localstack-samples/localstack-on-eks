package errors

import (
	"errors"

	"k8s.io/client-go/tools/record"
)

func NewWithRecorder(recorder record.EventRecorder, text string) error {
	recorder.Event(nil, "Warning", "Error", text)
	return errors.New(text)
}
