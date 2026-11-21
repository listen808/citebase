package log

import (
	"io"
	"strings"
	"testing"
)

func TestWriter_Default(t *testing.T) {
	w := Writer()
	if w == nil {
		t.Fatal("Writer() should not return nil")
	}
}

func TestStdLogger_Write(t *testing.T) {
	sl := StdLogger("[test] ", 0)
	if sl == nil {
		t.Fatal("StdLogger() should not return nil")
	}

	n, err := sl.Write([]byte("hello world\n"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != 12 {
		t.Errorf("Expected n=12, got %d", n)
	}
}

func TestStdLogger_WriteWithoutNewline(t *testing.T) {
	sl := StdLogger("[prefix] ", 0)
	n, err := sl.Write([]byte("no newline"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != 10 {
		t.Errorf("Expected n=10, got %d", n)
	}
}

func TestStdLogger_AsIOWriter(t *testing.T) {
	var _ io.Writer = StdLogger("", 0) // compile-time interface check
}

func TestInfo_DoesNotPanic(t *testing.T) {
	Info("test info message")
	Infof("test info %s", "formatted")
}

func TestWarn_DoesNotPanic(t *testing.T) {
	Warn("test warn message")
	Warnf("test warn %s", "formatted")
}

func TestError_DoesNotPanic(t *testing.T) {
	Error("test error message")
	Errorf("test error %s", "formatted")
}

func TestPanic_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("Expected Panic to panic")
		}
		if r != "test panic" {
			t.Errorf("Expected panic value 'test panic', got %v", r)
		}
	}()
	Panic("test panic")
}

func TestPanicf_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("Expected Panicf to panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "value=42") {
			t.Errorf("Expected panic message containing 'value=42', got %v", r)
		}
	}()
	Panicf("test panic value=%d", 42)
}

func TestInitFileLogger(t *testing.T) {
	InitFileLogger()

	w := Writer()
	if w == nil {
		t.Fatal("Writer() should not return nil after InitFileLogger")
	}

	Info("test after init")
}
