/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//nolint:nosnakecase
package http

import (
	"fmt"
	"io"
	"net/http"

	httpMiddleware "github.com/dapr/dapr/pkg/middleware/http"

	"github.com/valyala/fasthttp"

	middleware "github.com/dapr/components-contrib/middleware"
	"github.com/dapr/dapr/pkg/components"
	"github.com/dapr/dapr/pkg/components/pluggable"
	proto "github.com/dapr/dapr/pkg/proto/components/v1"
	"github.com/dapr/kit/logger"
)

// commandHandler is the command handler for grpc middleware.
func commandHandler(h fasthttp.RequestHandler, ctx *fasthttp.RequestCtx, callback func(*proto.CommandResponse) error) func(req *proto.Command) error {
	return func(req *proto.Command) error {
		switch command := req.Command.(type) {
		case *proto.Command_GetReqBody:
			return callback(&proto.CommandResponse{
				Response: &proto.CommandResponse_GetReqBody{
					GetReqBody: &proto.GetRequestBodyCommandResponse{
						Data: ctx.Request.Body(),
					},
				},
			})
		case *proto.Command_GetRespBody:
			return callback(&proto.CommandResponse{
				Response: &proto.CommandResponse_GetRespBody{
					GetRespBody: &proto.GetResponseBodyCommandResponse{
						Data: ctx.Response.Body(),
					},
				},
			})
		case *proto.Command_GetRespHeaders:
			headers := make(map[string]string)

			ctx.Response.Header.VisitAll(func(key, value []byte) {
				headers[string(key[:])] = string(value[:])
			})

			return callback(&proto.CommandResponse{
				Response: &proto.CommandResponse_GetRespHeaders{
					GetRespHeaders: &proto.GetResponseHeadersCommandResponse{
						Headers: headers,
					},
				},
			})
		case *proto.Command_GetReqHeaders:
			headers := make(map[string]string)

			ctx.Request.Header.VisitAllInOrder(func(key, value []byte) {
				headers[string(key[:])] = string(value[:])
			})

			return callback(&proto.CommandResponse{
				Response: &proto.CommandResponse_GetReqHeaders{
					GetReqHeaders: &proto.GetRequestHeadersCommandResponse{
						Method:  string(ctx.Method()),
						Uri:     ctx.URI().String(),
						Headers: headers,
					},
				},
			})
		case *proto.Command_ExecNext:
			h(ctx)
		case *proto.Command_SetReqHeaders:
			setRequestHeaderCommand := command.SetReqHeaders
			for headerKey, headerValue := range setRequestHeaderCommand.Headers {
				ctx.Request.Header.Set(headerKey, headerValue)
			}

			if setRequestHeaderCommand.Method != "" {
				ctx.Request.Header.SetMethod(setRequestHeaderCommand.Method)
			}

			if setRequestHeaderCommand.Uri != "" {
				ctx.Request.Header.SetRequestURI(setRequestHeaderCommand.Uri)
			}
		case *proto.Command_SetRespHeaders:
			for headerKey, headerValue := range command.SetRespHeaders.Headers {
				ctx.Response.Header.Set(headerKey, headerValue)
			}
		case *proto.Command_SetRespStatus:
			ctx.SetStatusCode(int(command.SetRespStatus.StatusCode))
		case *proto.Command_SetReqBody:
			ctx.Request.SetBody(command.SetReqBody.Data)
		case *proto.Command_SetRespBody:
			ctx.SetBody(command.SetRespBody.Data)
		}
		return nil
	}
}

// newGRPCMiddleware
func newGRPCMiddleware(l logger.Logger, pc components.Pluggable) FactoryMethod {
	return func(metadata middleware.Metadata) (httpMiddleware.Middleware, error) {
		connector := pluggable.NewGRPCConnector(pc, proto.NewHttpMiddlewareClient)
		if err := connector.Dial(metadata.Name); err != nil {
			return nil, err
		}

		if _, err := connector.Client.Init(connector.Context, &proto.MiddlewareInitRequest{
			Metadata: &proto.MetadataRequest{
				Properties: metadata.Properties,
			},
		}); err != nil {
			return nil, err
		}

		return func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				client, err := connector.Client.Handle(ctx)
				if err != nil {
					ctx.Error(err.Error(), http.StatusInternalServerError)
					return
				}
				defer client.CloseSend()
				handleCommand := commandHandler(h, ctx, client.Send)

				for {
					select {
					// middleware has finished the execution
					case <-client.Context().Done():
						return
					// timeout has reached
					case <-ctx.Done():
						ctx.Error(fmt.Sprintf("timed out waiting for middleware '%s'", metadata.Name), http.StatusInternalServerError)
						return
					default:
						cmd, err := client.Recv()

						if err == io.EOF {
							return
						}

						if err != nil {
							ctx.Error(err.Error(), http.StatusInternalServerError)
							return
						}

						if err := handleCommand(cmd); err != nil {
							ctx.Error(err.Error(), http.StatusInternalServerError)
							return
						}
					}
				}
			}
		}, nil
	}
}

func init() {
	pluggable.AddRegistryFor(components.HTTPMiddleware, DefaultRegistry.RegisterComponent, newGRPCMiddleware)
}
