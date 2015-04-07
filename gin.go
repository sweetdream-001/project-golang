// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"html/template"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/gin-gonic/gin/render"
)

// Param is a single URL parameter, consisting of a key and a value.
type Param struct {
	Key   string
	Value string
}

// Params is a Param-slice, as returned by the router.
// The slice is ordered, the first URL parameter is also the first slice value.
// It is therefore safe to read values by the index.
type Params []Param

// ByName returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) ByName(name string) string {
	for _, entry := range ps {
		if entry.Key == name {
			return entry.Value
		}
	}
	return ""
}

var default404Body = []byte("404 page not found")
var default405Body = []byte("405 method not allowed")

type (
	HandlerFunc func(*Context)

	// Represents the web framework, it wraps the blazing fast httprouter multiplexer and a list of global middlewares.
	Engine struct {
		RouterGroup
		HTMLRender  render.Render
		pool        sync.Pool
		allNoRoute  []HandlerFunc
		allNoMethod []HandlerFunc
		noRoute     []HandlerFunc
		noMethod    []HandlerFunc
		trees       map[string]*node

		// Enables automatic redirection if the current route can't be matched but a
		// handler for the path with (without) the trailing slash exists.
		// For example if /foo/ is requested but a route only exists for /foo, the
		// client is redirected to /foo with http status code 301 for GET requests
		// and 307 for all other request methods.
		RedirectTrailingSlash bool

		// If enabled, the router tries to fix the current request path, if no
		// handle is registered for it.
		// First superfluous path elements like ../ or // are removed.
		// Afterwards the router does a case-insensitive lookup of the cleaned path.
		// If a handle can be found for this route, the router makes a redirection
		// to the corrected path with status code 301 for GET requests and 307 for
		// all other request methods.
		// For example /FOO and /..//Foo could be redirected to /foo.
		// RedirectTrailingSlash is independent of this option.
		RedirectFixedPath bool

		// If enabled, the router checks if another method is allowed for the
		// current route, if the current request can not be routed.
		// If this is the case, the request is answered with 'Method Not Allowed'
		// and HTTP status code 405.
		// If no other Method is allowed, the request is delegated to the NotFound
		// handler.
		HandleMethodNotAllowed bool
	}
)

// Returns a new blank Engine instance without any middleware attached.
// The most basic configuration
func New() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			Handlers:     nil,
			absolutePath: "/",
		},
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		trees: make(map[string]*node),
	}
	engine.RouterGroup.engine = engine
	engine.pool.New = func() interface{} {
		return engine.allocateContext()
	}
	return engine
}

// Returns a Engine instance with the Logger and Recovery already attached.
func Default() *Engine {
	engine := New()
	engine.Use(Recovery(), Logger())
	return engine
}

func (engine *Engine) allocateContext() (context *Context) {
	context = &Context{Engine: engine}
	context.Input = inputHolder{context: context}
	return
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	if IsDebugging() {
		r := &render.HTMLDebugRender{Glob: pattern}
		engine.HTMLRender = r
	} else {
		templ := template.Must(template.ParseGlob(pattern))
		engine.SetHTMLTemplate(templ)
	}
}

func (engine *Engine) LoadHTMLFiles(files ...string) {
	if IsDebugging() {
		r := &render.HTMLDebugRender{Files: files}
		engine.HTMLRender = r
	} else {
		templ := template.Must(template.ParseFiles(files...))
		engine.SetHTMLTemplate(templ)
	}
}

func (engine *Engine) SetHTMLTemplate(templ *template.Template) {
	engine.HTMLRender = render.HTMLRender{
		Template: templ,
	}
}

// Adds handlers for NoRoute. It return a 404 code by default.
func (engine *Engine) NoRoute(handlers ...HandlerFunc) {
	engine.noRoute = handlers
	engine.rebuild404Handlers()
}

func (engine *Engine) NoMethod(handlers ...HandlerFunc) {
	engine.noMethod = handlers
	engine.rebuild405Handlers()
}

func (engine *Engine) Use(middlewares ...HandlerFunc) {
	engine.RouterGroup.Use(middlewares...)
	engine.rebuild404Handlers()
	engine.rebuild405Handlers()
}

func (engine *Engine) rebuild404Handlers() {
	engine.allNoRoute = engine.combineHandlers(engine.noRoute)
}

func (engine *Engine) rebuild405Handlers() {
	engine.allNoMethod = engine.combineHandlers(engine.noMethod)
}

func (engine *Engine) handle(method, path string, handlers []HandlerFunc) {
	if path[0] != '/' {
		panic("path must begin with '/'")
	}
	root := engine.trees[method]
	if root == nil {
		root = new(node)
		engine.trees[method] = root
	}
	root.addRoute(path, handlers)
}

func (engine *Engine) Run(addr string) error {
	debugPrint("Listening and serving HTTP on %s\n", addr)
	return http.ListenAndServe(addr, engine)
}

func (engine *Engine) RunTLS(addr string, cert string, key string) error {
	debugPrint("Listening and serving HTTPS on %s\n", addr)
	return http.ListenAndServeTLS(addr, cert, key, engine)
}

// ServeHTTP makes the router implement the http.Handler interface.
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	context := engine.pool.Get().(*Context)
	context.writermem.reset(w)
	context.Request = req
	context.reset()

	engine.serveHTTPRequest(context)

	engine.pool.Put(context)
}

func (engine *Engine) serveHTTPRequest(context *Context) {
	httpMethod := context.Request.Method
	path := context.Request.URL.Path

	// Find root of the tree for the given HTTP method
	if root := engine.trees[httpMethod]; root != nil {
		// Find route in tree
		handlers, params, tsr := root.getValue(path, context.Params)
		// Dispatch if we found any handlers
		if handlers != nil {
			context.handlers = handlers
			context.Params = params
			context.Next()
			context.writermem.WriteHeaderNow()
			return

		} else if httpMethod != "CONNECT" && path != "/" {
			if engine.serveAutoRedirect(context, root, tsr) {
				return
			}
		}
	}

	if engine.HandleMethodNotAllowed {
		for method, root := range engine.trees {
			if method != httpMethod {
				if handlers, _, _ := root.getValue(path, nil); handlers != nil {
					context.handlers = engine.allNoMethod
					serveError(context, 405, default405Body)
					return
				}
			}
		}
	}
	context.handlers = engine.allNoMethod
	serveError(context, 404, default404Body)
}

func (engine *Engine) serveAutoRedirect(c *Context, root *node, tsr bool) bool {
	req := c.Request
	path := req.URL.Path
	code := 301 // Permanent redirect, request with GET method
	if req.Method != "GET" {
		code = 307
	}

	if tsr && engine.RedirectTrailingSlash {
		if len(path) > 1 && path[len(path)-1] == '/' {
			req.URL.Path = path[:len(path)-1]
		} else {
			req.URL.Path = path + "/"
		}
		http.Redirect(c.Writer, req, req.URL.String(), code)
		return true
	}

	// Try to fix the request path
	if engine.RedirectFixedPath {
		fixedPath, found := root.findCaseInsensitivePath(
			CleanPath(path),
			engine.RedirectTrailingSlash,
		)
		if found {
			req.URL.Path = string(fixedPath)
			http.Redirect(c.Writer, req, req.URL.String(), code)
			return true
		}
	}
	return false
}

func serveError(c *Context, code int, defaultMessage []byte) {
	c.writermem.status = code
	c.Next()
	if !c.Writer.Written() {
		if c.Writer.Status() == code {
			c.Data(-1, binding.MIMEPlain, defaultMessage)
		} else {
			c.Writer.WriteHeaderNow()
		}
	}
}
