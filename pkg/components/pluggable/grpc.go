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

package pluggable

import (
	"context"
	"fmt"

	"github.com/dapr/dapr/pkg/components"
	"github.com/dapr/dapr/utils"

	"github.com/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GRPCClient is any client that supports common pluggable grpc operations.
type GRPCClient interface {
	// Ping is for liveness purposes.
	Ping(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

// GRPCConnector is a connector that uses underlying gRPC protocol for common operations.
type GRPCConnector[TClient GRPCClient] struct {
	// Context is the component shared context
	Context context.Context
	// Cancel is used for cancelling inflight requests
	Cancel context.CancelFunc
	// Client is the proto client.
	Client        TClient
	pluggable     components.Pluggable
	conn          *grpc.ClientConn
	clientFactory func(grpc.ClientConnInterface) TClient
}

const (
	daprSocketFolderEnvVar = "DAPR_PLUGGABLE_COMPONENTS_SOCKETS_FOLDER"
	defaultSocketFolder    = "/var/run"
)

// socketsFolder is where the dapr sockets should be created.
var socketsFolder string

func init() {
	socketsFolder = utils.GetEnvOrElse(daprSocketFolderEnvVar, defaultSocketFolder)
}

// socketPathFor returns a unique socket for the given component.
// the socket path will be composed by the pluggable component, name, version and type plus the component name.
func (g *GRPCConnector[TClient]) socketPathFor(componentName string) string {
	return fmt.Sprintf("%s/dapr-%s.%s-%s-%s.sock", socketsFolder, g.pluggable.Type, g.pluggable.Name, g.pluggable.Version, componentName)
}

// Dial opens a grpcConnection and creates a new client instance.
func (g *GRPCConnector[TClient]) Dial(componentName string, additionalOpts ...grpc.DialOption) error {
	udsSocket := fmt.Sprintf("unix://%s", g.socketPathFor(componentName))
	log.Debugf("using socket defined at '%s' for the component '%s'", udsSocket, componentName)
	// TODO Add Observability middlewares monitoring/tracing
	additionalOpts = append(additionalOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	grpcConn, err := grpc.Dial(udsSocket, additionalOpts...)
	if err != nil {
		return errors.Wrapf(err, "unable to open GRPC connection using socket '%s'", udsSocket)
	}
	g.conn = grpcConn

	g.Client = g.clientFactory(grpcConn)

	return g.Ping()
}

// Ping pings the grpc component.
// It uses "WaitForReady" avoiding failing in transient failures.
func (g *GRPCConnector[TClient]) Ping() error {
	_, err := g.Client.Ping(g.Context, &emptypb.Empty{}, grpc.WaitForReady(true))
	return err
}

// Close closes the underlying gRPC connection and cancel all inflight requests.
func (g *GRPCConnector[TClient]) Close() error {
	g.Cancel()

	return g.conn.Close()
}

// NewGRPCConnector creates a new grpc connector for the given client.
func NewGRPCConnector[TClient GRPCClient](pc components.Pluggable, factory func(grpc.ClientConnInterface) TClient) *GRPCConnector[TClient] {
	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCConnector[TClient]{
		Context:       ctx,
		Cancel:        cancel,
		pluggable:     pc,
		clientFactory: factory,
	}
}