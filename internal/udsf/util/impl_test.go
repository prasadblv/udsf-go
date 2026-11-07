package util

import "testing"

func TestResponse(t *testing.T) {
	r := Response(201, "body")
	if r.Code != 201 || r.Body != "body" {
		t.Errorf("unexpected response %+v", r)
	}
	if r := Response(204, nil); r.Body != nil || r.Code != 204 {
		t.Errorf("unexpected response %+v", r)
	}
}
