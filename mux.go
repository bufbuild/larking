// Copyright 2021 Edward McFarlane. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package larking

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type methodDesc struct {
	name string
	desc protoreflect.MethodDescriptor
}

// RO
type methodConn struct {
	methodDesc
	cc *grpc.ClientConn
}

// RO
type connList struct {
	descs  []methodDesc
	fdHash []byte
}

type state struct {
	path    *path
	conns   map[*grpc.ClientConn]connList
	methods map[string][]methodConn
}

func (s *state) clone() *state {
	if s == nil {
		return &state{
			path:    newPath(),
			conns:   make(map[*grpc.ClientConn]connList),
			methods: make(map[string][]methodConn),
		}
	}

	conns := make(map[*grpc.ClientConn]connList)
	for conn, cl := range s.conns {
		conns[conn] = cl
	}

	methods := make(map[string][]methodConn)
	for method, mcs := range s.methods {
		methods[method] = mcs
	}

	return &state{
		path:    s.path.clone(),
		conns:   conns,
		methods: methods,
	}
}

type muxOptions struct {
	maxReceiveMessageSize int
	maxSendMessageSize    int
	connectionTimeout     time.Duration
}

type MuxOption func(*muxOptions)

var defaultMuxOptions = muxOptions{
	maxReceiveMessageSize: defaultServerMaxReceiveMessageSize,
	maxSendMessageSize:    defaultServerMaxSendMessageSize,
	connectionTimeout:     defaultServerConnectionTimeout,
}

type Mux struct {
	opts  muxOptions
	mu    sync.Mutex   // Lock to sync writers
	state atomic.Value // Value of *state
}

func NewMux(opts ...MuxOption) (*Mux, error) {
	// Apply options.
	var muxOpts = defaultMuxOptions
	for _, opt := range opts {
		opt(&muxOpts)
	}

	return &Mux{
		opts: muxOpts,
	}, nil
}

func (m *Mux) RegisterConn(ctx context.Context, cc *grpc.ClientConn) error {
	c := rpb.NewServerReflectionClient(cc)

	// TODO: watch the stream. When it is recreated refresh the service
	// methods and recreate the mux if needed.
	fmt.Println("reflection?")
	stream, err := c.ServerReflectionInfo(ctx, grpc.WaitForReady(true))
	if err != nil {
		return err
	}
	fmt.Println("got stream")

	// Load the state for writing.
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.loadState().clone()

	if err := s.createHandler(cc, stream); err != nil {
		return err
	}

	m.storeState(s)

	return stream.CloseSend()
}

func (m *Mux) DropConn(ctx context.Context, cc *grpc.ClientConn) bool {
	// Load the state for writing.
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.loadState().clone()

	return s.removeHandler(cc)
}

// resolver implements protodesc.Resolver.
type resolver struct {
	files  protoregistry.Files
	stream rpb.ServerReflection_ServerReflectionInfoClient
}

func newResolver(stream rpb.ServerReflection_ServerReflectionInfoClient) (*resolver, error) {
	r := &resolver{stream: stream}

	if err := r.files.RegisterFile(annotations.File_google_api_annotations_proto); err != nil {
		return nil, err
	}
	if err := r.files.RegisterFile(annotations.File_google_api_http_proto); err != nil {
		return nil, err
	}
	if err := r.files.RegisterFile(httpbody.File_google_api_httpbody_proto); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *resolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if fd, err := r.files.FindFileByPath(path); err == nil {
		return fd, nil // found file
	}

	if err := r.stream.Send(&rpb.ServerReflectionRequest{
		MessageRequest: &rpb.ServerReflectionRequest_FileByFilename{
			FileByFilename: path,
		},
	}); err != nil {
		return nil, err
	}

	fdr, err := r.stream.Recv()
	if err != nil {
		return nil, err
	}
	fdbs := fdr.GetFileDescriptorResponse().GetFileDescriptorProto()

	var f protoreflect.FileDescriptor
	for _, fdb := range fdbs {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdb, fdp); err != nil {
			return nil, err
		}

		file, err := protodesc.NewFile(fdp, r)
		if err != nil {
			return nil, err
		}
		// TODO: check duplicate file registry
		if err := r.files.RegisterFile(file); err != nil {
			return nil, err
		}
		if file.Path() == path {
			f = file
		}
	}
	if f == nil {
		return nil, fmt.Errorf("missing file descriptor %s", path)
	}
	return f, nil
}

func (r *resolver) FindDescriptorByName(fullname protoreflect.FullName) (protoreflect.Descriptor, error) {
	return r.files.FindDescriptorByName(fullname)
}

func (s *state) removeHandler(cc *grpc.ClientConn) bool {
	cl, ok := s.conns[cc]
	if !ok {
		return ok
	}

	// Drop methods on client conn.
	for _, md := range cl.descs {
		var mcs []methodConn

		for _, mc := range s.methods[md.name] {
			if mc.cc != cc {
				mcs = append(mcs, mc)
			}
		}
		if len(mcs) == 0 {
			delete(s.methods, md.name)
			s.path.delRule(md.name)
		} else {
			s.methods[md.name] = mcs
		}
	}
	// Drop conn on client conn.
	delete(s.conns, cc)
	return ok
}

func (s *state) createHandler(
	cc *grpc.ClientConn,
	stream rpb.ServerReflection_ServerReflectionInfoClient,
) error {
	// TODO: async fetch and mux creation.

	if err := stream.Send(&rpb.ServerReflectionRequest{
		MessageRequest: &rpb.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return err
	}

	r, err := stream.Recv()
	if err != nil {
		return err
	}
	// TODO: check r.GetErrorResponse()?

	// File descriptors hash for detecting updates. TODO: sort fds?
	h := sha256.New()

	fds := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, svc := range r.GetListServicesResponse().GetService() {
		if err := stream.Send(&rpb.ServerReflectionRequest{
			MessageRequest: &rpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: svc.GetName(),
			},
		}); err != nil {
			return err
		}

		fdr, err := stream.Recv()
		if err != nil {
			return err
		}

		fdbb := fdr.GetFileDescriptorResponse().GetFileDescriptorProto()

		for _, fdb := range fdbb {
			fd := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(fdb, fd); err != nil {
				return err
			}
			fds[fd.GetName()] = fd

			if _, err := h.Write(fdb); err != nil {
				return err
			}
		}
	}

	fdHash := h.Sum(nil)

	// Check if previous connection exists.
	if cl, ok := s.conns[cc]; ok {
		if bytes.Equal(cl.fdHash, fdHash) {
			return nil // nothing to do
		}

		// Drop and recreate below.
		s.removeHandler(cc)
	}

	rslvr, err := newResolver(stream)
	if err != nil {
		return err
	}

	var methods []methodDesc
	for _, fd := range fds {
		file, err := protodesc.NewFile(fd, rslvr)
		if err != nil {
			return err
		}

		ms, err := s.processFile(cc, file)
		if err != nil {
			// TODO: partial dregister?
			return err
		}
		methods = append(methods, ms...)
	}

	// Update methods list.
	s.conns[cc] = connList{
		descs:  methods,
		fdHash: fdHash,
	}
	for _, method := range methods {
		s.methods[method.name] = append(
			s.methods[method.name], methodConn{method, cc},
		)
	}
	return nil
}

func (s *state) processFile(cc *grpc.ClientConn, fd protoreflect.FileDescriptor) ([]methodDesc, error) {
	var methods []methodDesc

	sds := fd.Services()
	for i := 0; i < sds.Len(); i++ {
		sd := sds.Get(i)
		name := sd.FullName()

		mds := sd.Methods()
		for j := 0; j < mds.Len(); j++ {
			md := mds.Get(j)

			opts := md.Options() // TODO: nil check fails?

			rule := getExtensionHTTP(opts)
			if rule == nil {
				continue
			}

			method := fmt.Sprintf("/%s/%s", name, md.Name())
			if err := s.path.addRule(rule, md, method); err != nil {
				// TODO: partial service registration?
				return nil, err
			}
			methods = append(methods, methodDesc{
				name: method,
				desc: md,
			})
		}
	}
	return methods, nil
}

func (m *Mux) loadState() *state {
	s, _ := m.state.Load().(*state)
	return s
}
func (m *Mux) storeState(s *state) { m.state.Store(s) }

func (s *state) pickMethodConn(name string) (methodConn, error) {
	mcs := s.methods[name]
	if len(mcs) == 0 {
		return methodConn{}, status.Errorf(
			codes.Unimplemented,
			fmt.Sprintf("method %s not implemented", name),
		)
	}
	mc := mcs[rand.Intn(len(mcs))]
	return mc, nil
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.serveHTTP(w, r)
}
