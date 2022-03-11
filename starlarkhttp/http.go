// Copyright 2021 Edward McFarlane. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package starlarkhttp

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/emcfarlane/larking/starext"
	"github.com/emcfarlane/larking/starlarkerrors"
	"github.com/emcfarlane/larking/starlarkio"
	"github.com/emcfarlane/larking/starlarkthread"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// NewModule loads the predeclared built-ins for the net/http module.
func NewModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "http",
		Members: starlark.StringDict{
			"default_client": defaultClient,
			"get":            starext.MakeBuiltin("http.do", defaultClient.get),
			"head":           starext.MakeBuiltin("http.head", defaultClient.head),
			"post":           starext.MakeBuiltin("http.post", defaultClient.post),
			"new_client":     starext.MakeBuiltin("http.new_client", NewClient),
			"new_request":    starext.MakeBuiltin("http.new_request", NewRequest),

			// net/http errors
			"err_not_supported":         starlarkerrors.NewError(http.ErrNotSupported),
			"err_missing_boundary":      starlarkerrors.NewError(http.ErrMissingBoundary),
			"err_not_multipart":         starlarkerrors.NewError(http.ErrNotMultipart),
			"err_body_not_allowed":      starlarkerrors.NewError(http.ErrBodyNotAllowed),
			"err_hijacked":              starlarkerrors.NewError(http.ErrHijacked),
			"err_content_length":        starlarkerrors.NewError(http.ErrContentLength),
			"err_abort_handler":         starlarkerrors.NewError(http.ErrAbortHandler),
			"err_body_read_after_close": starlarkerrors.NewError(http.ErrBodyReadAfterClose),
			"err_handler_timeout":       starlarkerrors.NewError(http.ErrHandlerTimeout),
			"err_line_too_long":         starlarkerrors.NewError(http.ErrLineTooLong),
			"err_missing_file":          starlarkerrors.NewError(http.ErrMissingFile),
			"err_no_cookie":             starlarkerrors.NewError(http.ErrNoCookie),
			"err_no_location":           starlarkerrors.NewError(http.ErrNoLocation),
			"err_server_closed":         starlarkerrors.NewError(http.ErrServerClosed),
			"err_skip_alt_protocol":     starlarkerrors.NewError(http.ErrSkipAltProtocol),
			"err_use_last_response":     starlarkerrors.NewError(http.ErrUseLastResponse),
		},
	}
}

type Client struct {
	Client *http.Client
	frozen bool
}

func MakeClient(client *http.Client) *Client {
	return &Client{Client: client}
}

func (v *Client) String() string        { return "<client>" }
func (v *Client) Type() string          { return "http.client" }
func (v *Client) Freeze()               { v.frozen = true }
func (v *Client) Truth() starlark.Bool  { return true }
func (v *Client) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", v.Type()) }

type clientAttr func(*Client) starlark.Value

var clientAttrs = map[string]clientAttr{
	"do": func(v *Client) starlark.Value { return starext.MakeMethod(v, "do", v.do) },
}

func (v *Client) Attr(name string) (starlark.Value, error) {
	if a := clientAttrs[name]; a != nil {
		return a(v), nil
	}
	return nil, nil
}
func (v *Client) AttrNames() []string {
	names := make([]string, 0, len(clientAttrs))
	for name := range clientAttrs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var defaultClient = &Client{Client: http.DefaultClient, frozen: true}

func (v *Client) get(thread *starlark.Thread, name string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlstr string
	if err := starlark.UnpackArgs("nethttp.get", args, kwargs,
		"url", &urlstr,
	); err != nil {
		return nil, err
	}

	response, err := v.Client.Get(urlstr)
	if err != nil {
		return nil, err
	}
	return makeResponse(thread, response)
}

func (v *Client) head(thread *starlark.Thread, name string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlstr string
	if err := starlark.UnpackArgs(name, args, kwargs,
		"url", &urlstr,
	); err != nil {
		return nil, err
	}

	response, err := v.Client.Head(urlstr)
	if err != nil {
		return nil, err
	}
	return makeResponse(thread, response)
}

func (v *Client) post(thread *starlark.Thread, name string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		urlstr      string
		contentType string
		body        starlark.Value
	)
	if err := starlark.UnpackArgs(name, args, kwargs,
		"url", &urlstr, "content_type", &contentType, "body", &body,
	); err != nil {
		return nil, err
	}
	r, err := makeReader(body)
	if err != nil {
		return nil, err
	}

	response, err := v.Client.Post(urlstr, contentType, r)
	if err != nil {
		return nil, err
	}
	return makeResponse(thread, response)
}

func NewClient(_ *starlark.Thread, name string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var client http.Client
	// TODO: implementation
	if err := starlark.UnpackPositionalArgs(name, args, kwargs, 0); err != nil {
		return nil, err
	}

	return &Client{
		Client: &client,
	}, nil
}

func (v *Client) do(thread *starlark.Thread, name string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var req Request
	if err := starlark.UnpackArgs(name, args, kwargs,
		"req", &req,
	); err != nil {
		return nil, err
	}

	response, err := v.Client.Do(req.request)
	if err != nil {
		return nil, err
	}

	rsp := &Response{
		response: response,
	}
	if err := starlarkthread.AddResource(thread, rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

type Request struct {
	request *http.Request
	frozen  bool
}

func makeReader(v starlark.Value) (io.Reader, error) {
	switch x := v.(type) {
	case starlark.String:
		return strings.NewReader(string(x)), nil
	case starlark.Bytes:
		return strings.NewReader(string(x)), nil
	case io.Reader:
		return x, nil
	case starlark.NoneType, nil:
		return http.NoBody, nil // none
	default:
		return nil, fmt.Errorf("unsupport type: %T", v)
	}
}

func NewRequest(thread *starlark.Thread, name string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		method string
		urlstr string
		body   starlark.Value
	)
	if err := starlark.UnpackArgs(name, args, kwargs,
		"method", &method, "url", &urlstr, "body?", &body,
	); err != nil {
		return nil, err
	}

	r, err := makeReader(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(method, urlstr, r)
	if err != nil {
		return nil, err
	}
	ctx := starlarkthread.Context(thread)
	request = request.WithContext(ctx)

	return &Request{
		request: request,
	}, nil

}

func (v *Request) String() string {
	return fmt.Sprintf("<request %s %s>", v.request.Method, v.request.URL.String())
}
func (v *Request) Type() string          { return "nethttp.request" }
func (v *Request) Freeze()               { v.frozen = true }
func (v *Request) Truth() starlark.Bool  { return v.request != nil }
func (v *Request) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", v.Type()) }

type Response struct {
	response *http.Response
	frozen   bool
}

func makeResponse(thread *starlark.Thread, response *http.Response) (*Response, error) {
	rsp := &Response{response: response}
	if err := starlarkthread.AddResource(thread, rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (v *Response) Close() error          { return v.response.Body.Close() }
func (v *Response) String() string        { return fmt.Sprintf("<response %s>", v.response.Status) }
func (v *Response) Type() string          { return "nethttp.response" }
func (v *Response) Freeze()               { v.frozen = true }
func (v *Response) Truth() starlark.Bool  { return v.response != nil }
func (v *Response) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", v.Type()) }

// TODO: optional methods io.Closer, etc.
var responseFields = map[string]func(v *Response) (starlark.Value, error){
	"body": func(v *Response) (starlark.Value, error) {
		return &starlarkio.Reader{Reader: v.response.Body}, nil
	},
}

func (v *Response) Attr(name string) (starlark.Value, error) {
	fn, ok := responseFields[name]
	if !ok {
		return nil, nil
	}
	return fn(v)
}
func (v *Response) AttrNames() []string {
	names := make([]string, 0, len(responseFields))
	for name := range responseFields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
