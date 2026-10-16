package order_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/shop/internal/order"
)

func newTestServer(t *testing.T, gw *fakeGateway) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	order.NewHandler(newTestService(t, gw), logger).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// result is a finished response: status, headers and the whole body.
type result struct {
	status int
	header http.Header
	body   []byte
}

// send performs a request and returns the response with its body fully
// read and closed, so tests never leak connections.
func send(t *testing.T, method, url, body string) result {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{status: resp.StatusCode, header: resp.Header, body: b}
}

func TestHTTPPlaceAndGet(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, &fakeGateway{})

	res := send(t, "POST", srv.URL+"/orders", `{"lines":[{"sku":"TEA","quantity":2}]}`)
	if res.status != http.StatusCreated {
		t.Fatalf("POST /orders status = %d, want %d; body %s", res.status, http.StatusCreated, res.body)
	}
	var created struct {
		ID         string `json:"id"`
		TotalCents int64  `json:"total_cents"`
	}
	if err := json.Unmarshal(res.body, &created); err != nil {
		t.Fatal(err)
	}
	if created.TotalCents != 900 {
		t.Errorf("total_cents = %d, want 900", created.TotalCents)
	}

	location := res.header.Get("Location")
	if got := send(t, "GET", srv.URL+location, ""); got.status != http.StatusOK {
		t.Errorf("GET %s status = %d, want %d", location, got.status, http.StatusOK)
	}
}

func TestHTTPErrors(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, &fakeGateway{})
	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantCode                 string
	}{
		{"malformed json", "POST", "/orders", `{"lines":`, http.StatusBadRequest, "request.malformed"},
		{"unknown field", "POST", "/orders", `{"lines":[],"coupon":"x"}`, http.StatusBadRequest, "request.malformed"},
		{"validation", "POST", "/orders", `{"lines":[]}`, http.StatusUnprocessableEntity, "order.invalid"},
		{"unknown product", "POST", "/orders", `{"lines":[{"sku":"NOPE","quantity":1}]}`, http.StatusUnprocessableEntity, "order.unknown_product"},
		{"missing order", "GET", "/orders/missing", "", http.StatusNotFound, "order.not_found"},
		{"body too large", "POST", "/orders", `{"lines":[{"sku":"` + strings.Repeat("x", 2<<20) + `"}]}`, http.StatusRequestEntityTooLarge, "request.too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := send(t, tt.method, srv.URL+tt.path, tt.body)
			if res.status != tt.wantStatus {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.path, res.status, tt.wantStatus)
			}
			var problem struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(res.body, &problem); err != nil {
				t.Fatalf("error body %q is not JSON: %v", res.body, err)
			}
			if problem.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", problem.Code, tt.wantCode)
			}
		})
	}
}

func TestHTTPPaymentFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		fail       error
		wantStatus int
		wantCode   string
	}{
		{"provider outage", errors.New("provider says: card 4242 timeout"), http.StatusBadGateway, "order.payment_failed"},
		{"declined", declinedErr{}, http.StatusPaymentRequired, "order.payment_declined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, &fakeGateway{fail: tt.fail})
			res := send(t, "POST", srv.URL+"/orders", `{"lines":[{"sku":"TEA","quantity":1}]}`)
			if res.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.status, tt.wantStatus)
			}
			var problem struct{ Code string }
			if err := json.Unmarshal(res.body, &problem); err != nil || problem.Code != tt.wantCode {
				t.Errorf("body %s: want code %q", res.body, tt.wantCode)
			}
			if got := res.header.Get("Location"); got != "/orders/ord-1" {
				t.Errorf("Location = %q, want /orders/ord-1", got)
			}
			if strings.Contains(string(res.body), "4242") {
				t.Errorf("response leaks provider detail: %s", res.body)
			}
		})
	}
}

func TestHTTPPayRetry(t *testing.T) {
	t.Parallel()
	gw := &fakeGateway{fail: errors.New("connection reset")}
	srv := newTestServer(t, gw)

	if res := send(t, "POST", srv.URL+"/orders", `{"lines":[{"sku":"TEA","quantity":1}]}`); res.status != http.StatusBadGateway {
		t.Fatalf("POST /orders status = %d, want %d", res.status, http.StatusBadGateway)
	}
	gw.setFail(nil)
	res := send(t, "POST", srv.URL+"/orders/ord-1/pay", "")
	if res.status != http.StatusOK || !strings.Contains(string(res.body), `"status":"paid"`) {
		t.Errorf("POST /orders/ord-1/pay = %d %s, want 200 and paid", res.status, res.body)
	}
}

func TestHTTPPlaceHonoursIdempotencyKey(t *testing.T) {
	t.Parallel()
	gw := &fakeGateway{}
	srv := newTestServer(t, gw)

	post := func() result {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), "POST", srv.URL+"/orders",
			strings.NewReader(`{"lines":[{"sku":"TEA","quantity":1}]}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Idempotency-Key", "retry-me")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return result{status: resp.StatusCode, header: resp.Header, body: b}
	}

	first, second := post(), post()
	if first.status != http.StatusCreated || second.status != http.StatusCreated {
		t.Fatalf("statuses = %d, %d; want 201 twice", first.status, second.status)
	}
	if first.header.Get("Location") != second.header.Get("Location") || gw.calls != 1 {
		t.Errorf("Locations %q and %q after %d charges; want the same order and 1 charge",
			first.header.Get("Location"), second.header.Get("Location"), gw.calls)
	}
}

func TestHTTPIdempotencyKeyMisuse(t *testing.T) {
	t.Parallel()
	post := func(t *testing.T, srv string, body string) result {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), "POST", srv+"/orders", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Idempotency-Key", "k1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return result{status: resp.StatusCode, header: resp.Header, body: b}
	}

	t.Run("different body", func(t *testing.T) {
		t.Parallel()
		srv := newTestServer(t, &fakeGateway{})
		post(t, srv.URL, `{"lines":[{"sku":"TEA","quantity":1}]}`)
		res := post(t, srv.URL, `{"lines":[{"sku":"MUG","quantity":1}]}`)
		if res.status != http.StatusUnprocessableEntity || !strings.Contains(string(res.body), "request.idempotency_key_reused") {
			t.Errorf("got %d %s, want 422 request.idempotency_key_reused", res.status, res.body)
		}
	})

	t.Run("unpaid original", func(t *testing.T) {
		t.Parallel()
		srv := newTestServer(t, &fakeGateway{fail: errors.New("connection reset")})
		post(t, srv.URL, `{"lines":[{"sku":"TEA","quantity":1}]}`)
		res := post(t, srv.URL, `{"lines":[{"sku":"TEA","quantity":1}]}`)
		if res.status != http.StatusConflict || res.header.Get("Location") != "/orders/ord-1" {
			t.Errorf("got %d Location %q, want 409 with /orders/ord-1", res.status, res.header.Get("Location"))
		}
	})
}
