// mongodb_exporter
// Copyright (C) 2026 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	wireOpMsg       = 2013
	wireHeaderLen   = 16
	wireOpMsgPrefix = wireHeaderLen + 5 // header, flagBits and the section kind byte
)

// dropConnection, returned by a reject function, makes the proxy close the client connection
// instead of answering, like a network failure.
const dropConnection = "\x00drop connection"

// strictProxy is a MongoDB wire protocol proxy that answers some commands with an error instead
// of forwarding them, like the proxies some managed services put in front of MongoDB
// (for example "Unallowed argument in listDatabases command: authorizedDatabases").
type strictProxy struct {
	addr     string
	rejected atomic.Int32
}

// startStrictProxy starts a strictProxy to upstream. reject receives the body of every OP_MSG
// command and returns the error message to answer with, or "" to forward the command.
func startStrictProxy(ctx context.Context, t *testing.T, upstream string, reject func(cmd bson.Raw) string) *strictProxy {
	t.Helper()

	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p := &strictProxy{addr: ln.Addr().String()}

	var (
		mu    sync.Mutex
		conns []net.Conn
	)
	track := func(c net.Conn) {
		mu.Lock()
		conns = append(conns, c)
		mu.Unlock()
	}

	t.Cleanup(func() {
		ln.Close() //nolint:errcheck
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close() //nolint:errcheck
		}
	})

	go func() {
		for {
			client, err := ln.Accept()
			if err != nil {
				return
			}
			server, err := new(net.Dialer).DialContext(ctx, "tcp", upstream)
			if err != nil {
				client.Close() //nolint:errcheck

				continue
			}
			track(client)
			track(server)

			go io.Copy(client, server) //nolint:errcheck
			go p.serve(client, server, reject)
		}
	}()

	return p
}

func (p *strictProxy) serve(client, server net.Conn, reject func(cmd bson.Raw) string) {
	for {
		msg, err := readWireMessage(client)
		if err != nil {
			return
		}

		if errmsg := commandRejection(msg, reject); errmsg != "" {
			p.rejected.Add(1)

			if errmsg == dropConnection {
				client.Close() //nolint:errcheck

				return
			}

			_, err = client.Write(wireErrorReply(msg, errmsg))
			if err != nil {
				return
			}

			continue
		}

		_, err = server.Write(msg)
		if err != nil {
			return
		}
	}
}

func readWireMessage(r io.Reader) ([]byte, error) {
	size := make([]byte, 4)

	_, err := io.ReadFull(r, size)
	if err != nil {
		return nil, fmt.Errorf("cannot read message size: %w", err)
	}

	msg := make([]byte, binary.LittleEndian.Uint32(size))
	copy(msg, size)

	_, err = io.ReadFull(r, msg[4:])
	if err != nil {
		return nil, fmt.Errorf("cannot read message: %w", err)
	}

	return msg, nil
}

func commandRejection(msg []byte, reject func(cmd bson.Raw) string) string {
	if len(msg) <= wireOpMsgPrefix || binary.LittleEndian.Uint32(msg[12:]) != wireOpMsg || msg[20] != 0 {
		return ""
	}

	body := bson.Raw(msg[wireOpMsgPrefix:])

	err := body.Validate()
	if err != nil {
		return ""
	}

	return reject(body)
}

func wireErrorReply(request []byte, errmsg string) []byte {
	doc, _ := bson.Marshal(bson.D{
		{Key: "ok", Value: 0.0},
		{Key: "errmsg", Value: errmsg},
		{Key: "code", Value: int32(8000)},
		{Key: "codeName", Value: "AtlasError"},
	})

	reply := make([]byte, wireOpMsgPrefix, wireOpMsgPrefix+len(doc))
	reply = append(reply, doc...)
	binary.LittleEndian.PutUint32(reply[0:], uint32(len(reply))) //nolint:gosec
	copy(reply[8:12], request[4:8])                              // responseTo = request ID
	binary.LittleEndian.PutUint32(reply[12:], wireOpMsg)

	return reply
}

// rejectArgs returns a reject function for startStrictProxy that rejects the command named cmd
// when it carries any of args, or always when args is empty.
func rejectArgs(cmd string, args ...string) func(bson.Raw) string {
	return func(body bson.Raw) string {
		elems, _ := body.Elements()
		if len(elems) == 0 || !strings.EqualFold(elems[0].Key(), cmd) {
			return ""
		}
		if len(args) == 0 {
			return "command " + cmd + " is unsupported"
		}
		for _, e := range elems[1:] {
			for _, arg := range args {
				if e.Key() == arg {
					return "Unallowed argument in " + cmd + " command: " + arg
				}
			}
		}

		return ""
	}
}

// commandName returns the name of the command in body, or "" if body is empty.
func commandName(body bson.Raw) string {
	elems, _ := body.Elements()
	if len(elems) == 0 {
		return ""
	}

	return elems[0].Key()
}

// rejectAny combines reject functions; the first rejection wins.
func rejectAny(fns ...func(bson.Raw) string) func(bson.Raw) string {
	return func(body bson.Raw) string {
		for _, fn := range fns {
			if msg := fn(body); msg != "" {
				return msg
			}
		}

		return ""
	}
}
