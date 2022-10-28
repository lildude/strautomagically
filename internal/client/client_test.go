package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

var baseURL = &url.URL{Scheme: "http", Host: "example.com", Path: "/"}

// TestNewClient confirms that a client can be created with the default baseURL
// and default User-Agent.
func TestNewClient(t *testing.T) {
	c := NewClient(baseURL, nil)

	if c.BaseURL.String() != baseURL.String() {
		t.Errorf("NewClient BaseURL is %v, expected %v", c.BaseURL, baseURL)
	}
	if c.userAgent != userAgent {
		t.Errorf("NewClient User-Agent is %v, expected %v", c.userAgent, userAgent)
	}
}

// TestNewRequest confirms that NewRequest returns an API request with the
// correct URL, a correctly encoded body and the correct User-Agent and
// Content-Type headers set.
func TestNewRequest(t *testing.T) {
	c := NewClient(baseURL, nil)

	// UserInfo is the information for the current user
	type TestInfo struct {
		Age    int     `json:"age"`
		Email  string  `json:"email"`
		Gender string  `json:"gender"`
		Height float64 `json:"height"`
		Weight float64 `json:"weight"`
	}

	t.Run("valid request", func(tc *testing.T) {
		inURL, outURL := "foo", baseURL.String()+"foo"
		inBody, outBody := &TestInfo{
			Age: 99, Weight: 102, Gender: "ano", Email: "user@example.com", Height: 184,
		}, `{"age":99,"email":"user@example.com","gender":"ano","height":184,"weight":102}`+"\n"

		req, err := c.NewRequest(context.Background(), "GET", inURL, inBody)
		if err != nil {
			tc.Errorf("Unexpected error: %s", err)
		}
		if req.URL.String() != outURL {
			tc.Errorf("Expecting URL %v, got %v", outURL, req.URL.String())
		}

		body, _ := io.ReadAll(req.Body)
		if string(body) != outBody {
			tc.Errorf("Expecting body %v, got %v", outBody, string(body))
		}
		if req.Header.Get("User-Agent") != userAgent {
			tc.Errorf("Expecting User-Agent %v, got %v", userAgent, req.Header.Get("User-Agent"))
		}
		if req.Header.Get("Content-Type") != "application/json" {
			tc.Errorf("Expecting Content-Type %v, got %v", "application/json", req.Header.Get("Content-Type"))
		}
	})

	t.Run("request with invalid JSON", func(tc *testing.T) {
		type T struct{ A map[interface{}]interface{} }
		_, err := c.NewRequest(context.Background(), "GET", ".", &T{})
		if err == nil {
			tc.Error("Expected error")
		}
	})

	t.Run("request with an invalid URL", func(tc *testing.T) {
		_, err := c.NewRequest(context.Background(), "GET", ":", nil)
		if err == nil {
			tc.Error("Expected error")
		}
	})

	t.Run("request with an invalid Method", func(tc *testing.T) {
		_, err := c.NewRequest(context.Background(), "\n", "/", nil)
		if err == nil {
			tc.Error("Expected error")
		}
	})

	t.Run("request with an empty body", func(tc *testing.T) {
		req, err := c.NewRequest(context.Background(), "GET", ".", nil)
		if err != nil {
			tc.Error("Unexpected error")
		}
		if req.Body != nil {
			tc.Error("Expected nil body")
		}
	})
}

// TestDo confirms that Do returns a JSON decoded value when making a request. It
// confirms the correct verb was used and that the decoded response value matches
// the expected result.
func TestDo(t *testing.T) {
	t.Run("successful GET request", func(tc *testing.T) {
		client, mux, teardown := setup()
		defer teardown()

		type foo struct{ A string }

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			fmt.Fprint(w, `{"A":"a"}`)
		})

		want := &foo{"a"}
		got := new(foo)

		req, _ := client.NewRequest(context.Background(), "GET", ".", nil)
		client.Do(req, got) //nolint:errcheck,bodyclose // we don't care about this in tests

		if !reflect.DeepEqual(got, want) {
			t.Errorf("Expecting %v, got %v", want, got)
		}
	})

	t.Run("GET request that returns an HTTP error", func(tc *testing.T) {
		client, mux, teardown := setup()
		defer teardown()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.WriteHeader(http.StatusInternalServerError)
		})

		req, _ := client.NewRequest(context.Background(), "GET", ".", nil)
		resp, err := client.Do(req, nil) //nolint:bodyclose // we don't care about this in tests

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expecting status code %v, got %v", http.StatusInternalServerError, resp.StatusCode)
		}
		if err == nil {
			t.Errorf("Expected error")
		}
	})

	t.Run("GET request that receives an empty payload", func(tc *testing.T) {
		client, mux, teardown := setup()
		defer teardown()

		type foo struct{ A string }

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		})

		req, _ := client.NewRequest(context.Background(), "GET", ".", nil)
		got := new(foo)
		resp, err := client.Do(req, got) //nolint:bodyclose // we don't care about this in tests

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expecting status code %v, got %v", http.StatusOK, resp.StatusCode)
		}
		if err != nil {
			t.Error("Unexpected error")
		}
	})

	t.Run("GET request that receives an HTML response", func(tc *testing.T) {
		client, mux, teardown := setup()
		defer teardown()

		type foo struct{ A string }

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			html := `<!doctype html>
			<html lang="en-GB">
			<head>
			  <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
			  <title>Default Page Title</title>
			  <link rel="shortcut icon" href="favicon.ico">
			  <link rel="icon" href="favicon.ico">
			  <link rel="stylesheet" type="text/css" href="styles.css">
			</head>

			<body>

			</body>
			</html>	`
			fmt.Fprintln(w, html)
		})

		req, _ := client.NewRequest(context.Background(), "GET", ".", nil)
		got := new(foo)
		resp, err := client.Do(req, got) //nolint:bodyclose // we don't care about this in tests

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expecting status code %v, got %v", http.StatusOK, resp.StatusCode)
		}
		if err == nil {
			t.Error("Expected error")
		}
	})

	t.Run("GET request on a cancelled context", func(tc *testing.T) {
		client, mux, teardown := setup()
		defer teardown()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req, _ := client.NewRequest(ctx, "GET", ".", nil)

		resp, err := client.Do(req, nil) //nolint:bodyclose // we don't care about this in tests

		if err == nil {
			t.Error("Expected error")
		}
		if resp != nil {
			t.Error("Expected nil response")
		}
	})

	t.Run("GET request that returns an error response", func(tc *testing.T) {
		client, mux, teardown := setup()
		defer teardown()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.WriteHeader(http.StatusBadRequest)
			resp := `{
				"detail": "Start date is greater than end date: [start_date: 2020-01-25; end_date: 2020-01-22]"
			}`
			fmt.Fprintln(w, resp)
		})

		req, _ := client.NewRequest(context.Background(), "GET", ".", nil)
		resp, err := client.Do(req, nil) //nolint:bodyclose // we don't care about this in tests

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expecting status code %v, got %v", http.StatusBadRequest, resp.StatusCode)
		}
		if err == nil {
			t.Error("Expected error")
		}
	})
}

// Setup establishes a test Server that can be used to provide mock responses during testing.
// It returns a pointer to a client, a mux, the server URL and a teardown function that
// must be called when testing is complete.
func setup() (client *Client, mux *http.ServeMux, teardown func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	surl, _ := url.Parse(server.URL + "/")
	c := NewClient(surl, nil)

	return c, mux, server.Close
}
