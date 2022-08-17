package runtime

import (
	components_v1alpha1 "github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	"github.com/dapr/dapr/pkg/components"
	"github.com/dapr/dapr/pkg/components/bindings"
	"github.com/dapr/dapr/pkg/components/configuration"
	"github.com/dapr/dapr/pkg/components/lock"
	"github.com/dapr/dapr/pkg/components/middleware/http"
	"github.com/dapr/dapr/pkg/components/nameresolution"
	"github.com/dapr/dapr/pkg/components/pluggable"
	"github.com/dapr/dapr/pkg/components/pubsub"
	"github.com/dapr/dapr/pkg/components/secretstores"
	"github.com/dapr/dapr/pkg/components/state"
)

type (
	// runtimeOpts encapsulates the components to include in the runtime.
	runtimeOpts struct {
		secretStores    []secretstores.SecretStore
		states          []state.State
		configurations  []configuration.Configuration
		locks           []lock.Lock
		pubsubs         []pubsub.PubSub
		nameResolutions []nameresolution.NameResolution
		inputBindings   []bindings.InputBinding
		outputBindings  []bindings.OutputBinding
		httpMiddleware  []http.Middleware

		componentsCallback ComponentsCallback
	}

	// Option is a function that customizes the runtime.
	Option func(o *runtimeOpts)
)

// WithSecretStores adds secret store components to the runtime.
func WithSecretStores(secretStores ...secretstores.SecretStore) Option {
	return func(o *runtimeOpts) {
		o.secretStores = append(o.secretStores, secretStores...)
	}
}

// WithStates adds state store components to the runtime.
func WithStates(states ...state.State) Option {
	return func(o *runtimeOpts) {
		o.states = append(o.states, states...)
	}
}

// WithConfigurations adds configuration store components to the runtime.
func WithConfigurations(configurations ...configuration.Configuration) Option {
	return func(o *runtimeOpts) {
		o.configurations = append(o.configurations, configurations...)
	}
}

func WithLocks(locks ...lock.Lock) Option {
	return func(o *runtimeOpts) {
		o.locks = append(o.locks, locks...)
	}
}

// WithPubSubs adds pubsub store components to the runtime.
func WithPubSubs(pubsubs ...pubsub.PubSub) Option {
	return func(o *runtimeOpts) {
		o.pubsubs = append(o.pubsubs, pubsubs...)
	}
}

// WithNameResolutions adds name resolution components to the runtime.
func WithNameResolutions(nameResolutions ...nameresolution.NameResolution) Option {
	return func(o *runtimeOpts) {
		o.nameResolutions = append(o.nameResolutions, nameResolutions...)
	}
}

// WithInputBindings adds input binding components to the runtime.
func WithInputBindings(inputBindings ...bindings.InputBinding) Option {
	return func(o *runtimeOpts) {
		o.inputBindings = append(o.inputBindings, inputBindings...)
	}
}

// WithOutputBindings adds output binding components to the runtime.
func WithOutputBindings(outputBindings ...bindings.OutputBinding) Option {
	return func(o *runtimeOpts) {
		o.outputBindings = append(o.outputBindings, outputBindings...)
	}
}

// WithHTTPMiddleware adds HTTP middleware components to the runtime.
func WithHTTPMiddleware(httpMiddleware ...http.Middleware) Option {
	return func(o *runtimeOpts) {
		o.httpMiddleware = append(o.httpMiddleware, httpMiddleware...)
	}
}

// WithComponentsCallback sets the components callback for applications that embed Dapr.
func WithComponentsCallback(componentsCallback ComponentsCallback) Option {
	return func(o *runtimeOpts) {
		o.componentsCallback = componentsCallback
	}
}

// withOpts applies all given options to runtime.
func withOpts(opts ...Option) Option {
	return func(runtimeOpts *runtimeOpts) {
		for _, opt := range opts {
			opt(runtimeOpts)
		}
	}
}

// pluggableLoaders maps a component type to its pluggable component loader.
var pluggableLoaders = make(map[components.Type]func(components_v1alpha1.PluggableComponent) Option)

func init() {
	withLoader(components.State, WithStates)
	withLoader(components.PubSub, WithPubSubs)
	withLoader(components.InputBinding, WithInputBindings)
	withLoader(components.OutputBinding, WithOutputBindings)
	withLoader(components.HTTPMiddleware, WithHTTPMiddleware)
	withLoader(components.Configuration, WithConfigurations)
	withLoader(components.Secret, WithSecretStores)
	withLoader(components.Lock, WithLocks)
	withLoader(components.NameResolution, WithNameResolutions)
}

// withLoader adds (or replace) a new pluggable loader to the loader map.
func withLoader[T any](componentType components.Type, add func(...T) Option) {
	pluggableLoaders[componentType] = func(pc components_v1alpha1.PluggableComponent) Option {
		return add(pluggable.MustLoad[T](pc))
	}
}

// WithPluggables parses and adds a new component into the target component list.
func WithPluggables(pluggables ...components_v1alpha1.PluggableComponent) Option {
	opts := make([]Option, 0)
	for _, pluggable := range pluggables {
		load, ok := pluggableLoaders[components.Type(pluggable.Spec.Type)]
		// ignoring unknown components
		if ok {
			opts = append(opts, load(pluggable))
		}
	}
	return withOpts(opts...)
}
