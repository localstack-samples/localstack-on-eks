package errors

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

func NewWithRecorder(recorder record.EventRecorder, object runtime.Object, text string) error {
	recorder.Event(object, "Warning", "Error", text)
	return errors.New(text)
}
