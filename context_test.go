package gin

import (
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestContextParamsGet tests that a parameter can be parsed from the URL.
func TestContextParamsByName(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test/alexandernyquist", nil)
	w := httptest.NewRecorder()
	name := ""

	r := Default()
	r.GET("/test/:name", func(c *Context) {
		name = c.Params.ByName("name")
	})

	r.ServeHTTP(w, req)

	if name != "alexandernyquist" {
		t.Errorf("Url parameter was not correctly parsed. Should be alexandernyquist, was %s.", name)
	}
}

// TestContextSetGet tests that a parameter is set correctly on the
// current context and can be retrieved using Get.
func TestContextSetGet(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r := Default()
	r.GET("/test", func(c *Context) {
		// Key should be lazily created
		if c.Keys != nil {
			t.Error("Keys should be nil")
		}

		// Set
		c.Set("foo", "bar")

		v, err := c.Get("foo")
		if err != nil {
			t.Errorf("Error on exist key")
		}
		if v != "bar" {
			t.Errorf("Value should be bar, was %s", v)
		}
	})

	r.ServeHTTP(w, req)
}

// TestContextJSON tests that the response is serialized as JSON
// and Content-Type is set to application/json
func TestContextJSON(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r := Default()
	r.GET("/test", func(c *Context) {
		c.JSON(200, H{"foo": "bar"})
	})

	r.ServeHTTP(w, req)

	if w.Body.String() != "{\"foo\":\"bar\"}\n" {
		t.Errorf("Response should be {\"foo\":\"bar\"}, was: %s", w.Body.String())
	}

	if w.HeaderMap.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type should be application/json, was %s", w.HeaderMap.Get("Content-Type"))
	}
}

// TestContextHTML tests that the response executes the templates
// and responds with Content-Type set to text/html
func TestContextHTML(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r := Default()
	templ, _ := template.New("t").Parse(`Hello {{.Name}}`)
	r.SetHTMLTemplate(templ)

	type TestData struct{ Name string }

	r.GET("/test", func(c *Context) {
		c.HTML(200, "t", TestData{"alexandernyquist"})
	})

	r.ServeHTTP(w, req)

	if w.Body.String() != "Hello alexandernyquist" {
		t.Errorf("Response should be Hello alexandernyquist, was: %s", w.Body.String())
	}

	if w.HeaderMap.Get("Content-Type") != "text/html" {
		t.Errorf("Content-Type should be text/html, was %s", w.HeaderMap.Get("Content-Type"))
	}
}

// TestContextString tests that the response is returned
// with Content-Type set to text/plain
func TestContextString(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r := Default()
	r.GET("/test", func(c *Context) {
		c.String(200, "test")
	})

	r.ServeHTTP(w, req)

	if w.Body.String() != "test" {
		t.Errorf("Response should be test, was: %s", w.Body.String())
	}

	if w.HeaderMap.Get("Content-Type") != "text/plain" {
		t.Errorf("Content-Type should be text/plain, was %s", w.HeaderMap.Get("Content-Type"))
	}
}

// TestContextXML tests that the response is serialized as XML
// and Content-Type is set to application/xml
func TestContextXML(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r := Default()
	r.GET("/test", func(c *Context) {
		c.XML(200, H{"foo": "bar"})
	})

	r.ServeHTTP(w, req)

	if w.Body.String() != "<map><foo>bar</foo></map>" {
		t.Errorf("Response should be <map><foo>bar</foo></map>, was: %s", w.Body.String())
	}

	if w.HeaderMap.Get("Content-Type") != "application/xml" {
		t.Errorf("Content-Type should be application/xml, was %s", w.HeaderMap.Get("Content-Type"))
	}
}

// TestContextData tests that the response can be written from `bytesting`
// with specified MIME type
func TestContextData(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test/csv", nil)
	w := httptest.NewRecorder()

	r := Default()
	r.GET("/test/csv", func(c *Context) {
		c.Data(200, "text/csv", []byte(`foo,bar`))
	})

	r.ServeHTTP(w, req)

	if w.Body.String() != "foo,bar" {
		t.Errorf("Response should be foo&bar, was: %s", w.Body.String())
	}

	if w.HeaderMap.Get("Content-Type") != "text/csv" {
		t.Errorf("Content-Type should be text/csv, was %s", w.HeaderMap.Get("Content-Type"))
	}
}


func TestContextFile(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test/file", nil)
	w := httptest.NewRecorder()

	r := Default()
	r.GET("/test/file", func(c *Context) {
		c.File("./gin.go")
	})

	r.ServeHTTP(w, req)

	bodyAsString := w.Body.String()

	if len(bodyAsString) == 0 {
		t.Errorf("Got empty body instead of file data")
	}

	if w.HeaderMap.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type should be text/plain; charset=utf-8, was %s", w.HeaderMap.Get("Content-Type"))
	}
}

// TestHandlerFunc - ensure that custom middleware works properly
func TestHandlerFunc(t *testing.T) {

	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r := Default()
	var stepsPassed int = 0

	r.Use(func(context *Context) {
		stepsPassed += 1
		context.Next()
		stepsPassed += 1
	})

	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("Response code should be Not found, was: %s", w.Code)
	}

	if stepsPassed != 2 {
		t.Errorf("Falied to switch context in handler function: %s", stepsPassed)
	}
}

// TestBadAbortHandlersChain - ensure that Abort after switch context will not interrupt pending handlers
func TestBadAbortHandlersChain(t *testing.T) {

	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r := Default()
	var stepsPassed int = 0

	r.Use(func(context *Context) {
		stepsPassed += 1
		context.Next()
		stepsPassed += 1
		// after check and abort
		context.Abort(409)
	},
		func(context *Context) {
			stepsPassed += 1
			context.Next()
			stepsPassed += 1
			context.Abort(403)
		},
	)

	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("Response code should be Forbiden, was: %s", w.Code)
	}

	if stepsPassed != 4 {
		t.Errorf("Falied to switch context in handler function: %s", stepsPassed)
	}
}

// TestAbortHandlersChain - ensure that Abort interrupt used middlewares in fifo order
func TestAbortHandlersChain(t *testing.T) {

	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r := Default()
	var stepsPassed int = 0

	r.Use(func(context *Context) {
		stepsPassed += 1
		context.Abort(409)
	},
		func(context *Context) {
			stepsPassed += 1
			context.Next()
			stepsPassed += 1
		},
	)

	r.ServeHTTP(w, req)

	if w.Code != 409 {
		t.Errorf("Response code should be Conflict, was: %s", w.Code)
	}

	if stepsPassed != 1 {
		t.Errorf("Falied to switch context in handler function: %s", stepsPassed)
	}
}

// TestFailHandlersChain - ensure that Fail interrupt used middlewares in fifo order as
// as well as Abort
func TestFailHandlersChain(t *testing.T) {

	req, _ := http.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r := Default()
	var stepsPassed int = 0

	r.Use(func(context *Context) {
		stepsPassed += 1

		context.Fail(500, errors.New("foo"))
	},
		func(context *Context) {
			stepsPassed += 1
			context.Next()
			stepsPassed += 1
		},
	)

	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("Response code should be Server error, was: %s", w.Code)
	}

	if stepsPassed != 1 {
		t.Errorf("Falied to switch context in handler function: %s", stepsPassed)
	}

}
