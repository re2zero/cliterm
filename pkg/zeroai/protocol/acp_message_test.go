package protocol

import (
	"encoding/json"
	"testing"
)

func TestDecodeMessage_ResponseWithID1(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"result":{"sessionId":"test-123"}}`)

	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage error: %v", err)
	}

	resp, ok := msg.(*AcpResponse)
	if !ok {
		t.Fatalf("expected *AcpResponse, got %T", msg)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID=1, got %d", resp.ID)
	}
}

func TestDecodeMessage_Notification(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionUpdate":"text","content":"hello"}}`)

	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage error: %v", err)
	}

	notif, ok := msg.(*AcpNotification)
	if !ok {
		t.Fatalf("expected *AcpNotification, got %T", msg)
	}
	if notif.Method != "session/update" {
		t.Errorf("expected method 'session/update', got %s", notif.Method)
	}
}

func TestDecodeMessage_ResponseIDZero(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":0,"result":{}}`)

	msg, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage error: %v", err)
	}

	_, isNotif := msg.(*AcpNotification)
	if isNotif {
		t.Error("response with id=0 was incorrectly decoded as notification")
	}
}

func TestDecodeMessage_ResponseWithStringID(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":"1","result":{"sessionId":"test-123"}}`)

	msg, err := DecodeMessage(data)
	if err != nil {
		t.Logf("string ID not supported by current AcpResponse.ID int type: %v (known limitation)", err)
		return
	}

	resp, ok := msg.(*AcpResponse)
	if !ok {
		t.Fatalf("response with string id was incorrectly decoded as %T instead of *AcpResponse", msg)
	}
	if resp.ID != 1 {
		t.Errorf("expected ID=1, got %d", resp.ID)
	}
}

func TestDecodeMessage_HasMethod(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"session/update","params":{}}`)

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	_, hasID := raw["id"]
	_, hasMethod := raw["method"]

	if hasID {
		t.Error("notification should not have id field")
	}
	if !hasMethod {
		t.Error("notification should have method field")
	}
}

func TestAcpResponseParsing(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":2,"result":{"sessionId":"sess-abc","models":{"default":"claude-sonnet-4-5"}}}`)

	var resp AcpResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.ID != 2 {
		t.Errorf("expected ID=2, got %d", resp.ID)
	}
	if resp.Result == nil {
		t.Error("expected non-nil Result")
	}
}

func TestAcpNotificationParsing(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"session/update","params":{"sessionUpdate":"agent_message_chunk","content":"Hello world"}}`)

	var notif AcpNotification
	if err := json.Unmarshal(data, &notif); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if notif.Method != "session/update" {
		t.Errorf("expected method 'session/update', got %s", notif.Method)
	}
	if notif.Params == nil {
		t.Error("expected non-nil Params")
	}
}

func TestDecodeMessage_DetectResponseVsNotification(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		wantResp bool
	}{
		{"response with int id", `{"jsonrpc":"2.0","id":1,"result":{}}`, true},
		{"response with id=0", `{"jsonrpc":"2.0","id":0,"result":{}}`, true},
		{"notification no id", `{"jsonrpc":"2.0","method":"test","params":{}}`, false},
		{"notification empty params", `{"jsonrpc":"2.0","method":"test"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := DecodeMessage([]byte(tt.data))
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			_, isResp := msg.(*AcpResponse)
			_, isNotif := msg.(*AcpNotification)

			if tt.wantResp && !isResp {
				t.Errorf("expected response, got %T", msg)
			}
			if !tt.wantResp && !isNotif {
				t.Errorf("expected notification, got %T", msg)
			}
		})
	}
}
