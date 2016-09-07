/*
Package di helps implement Dependency Injection for web development. Dependency
Injection aims to make dependencies accessible to its clients without them
having to construct or ask for their dependencies explicitly. This raises the
questions

	- Where should objects be constructed ?
	- How to make those objects available to all of the places it is needed ?

di uses factories to isolate dependency construction and make them available via
struct fields. This makes them accessible to code that uses them residing in
methods on those structs.

To that end di provides the types Dispatcher, Binding and interfaces
ApplicationFactory, RequestFactory, Controller and Router.

A Controller represents a type whose methods can serve HTTP requests and exports
Bindings. A Binding specifies binds an HTTP request with a specific <Verb,Path>
to a method on that Controller. A Controller can be registered with a Dispatcher
teaching it how to route requests.

ApplicationFactory and RequestFactory encapsulate all object construction
including Controllers. Dispatcher uses the factories to get hold of a Controller
object for the request and call the appropriate method.

The Dispatcher uses a Router to handle request multiplexing.

The flow of control while serving requests looks like

	- Request arrives.
	- Router routes it based on <Verb, Path> to a closure registered by the
	  Dispatcher.
	- The closure gets hold of RequestFactory by calling ApplicationFactory.With
	  passing it the request object.
	- The closure gets hold of Controller by passing the appropriate label to
	  RequestFactory.NewController .
	- The closure looks up and calls the Controller method registered.

The example demonstrates how to wire everything up.
*/
package di

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// An ApplicationFactory is expected to have access to all singletons and know
// how to create a RequestFactory given a request object.
type ApplicationFactory interface {
	// With associates a RequestFactory with the request object.
	With(*http.Request) RequestFactory
}

// A RequestFactory is expected to know to create Controllers along with their
// dependencies. The Dispatcher uses RequestFactory to get hold of the
// Controller to dispatch the request to.
type RequestFactory interface {
	// NewController returns a controller instance given the label it was
	// registered with. It is expected to panic if it encounters errors during
	// construction.
	NewController(label string) Controller
}

// A Binding describes how a request is to be routed and is returned by
// Controller.Bindings. It specifies that the request <Verb, Path> be delivered
// to the method Name. The method Name refers to is required to be of type
//
//		func(Controller, http.ResponseWriter, *http.Request)
//
// Reflection is used to lookup Name and validate it during registration.
type Binding struct {
	Verb string // The HTTP Verb to use
	Path string // The URL path to attach the method to
	Name string // Name of the method the request should be dispatched to
}

// A Controller has methods that handle requests. It exports Bindings describing
// how those methods are to be bound.
type Controller interface {
	Bindings() []Binding
}

// A Router represents the ability to multiplex an http request with <Verb,
// Path> to handler. The Dispatcher delegates request multiplexing to Router. A
// simple implementation around http.ServeMux is provided in sub-package router.
type Router interface {
	Handle(verb string, path string, handler http.Handler)
	HandleFunc(verb string, path string, handler func(http.ResponseWriter, *http.Request))

	// Must be a handler itself
	http.Handler
}

// Dispatcher orchestrates request handling with the help of the other types in
// this package. It uses Router to multiplex requests, ApplicationFactory and
// RequestFactory to get hold of fully constructed Controllers. It then
// dispatches the request to the appropriate Controller method.
type Dispatcher struct {
	name    string
	router  Router
	factory ApplicationFactory
}

// New creates a new Dispatcher. It panics if any of its arguments have zero
// values.
func New(name string, router Router, factory ApplicationFactory) Dispatcher {
	if name == "" {
		panic(errors.New("argument 'name' cannot be empty"))
	}
	if router == nil {
		panic(errors.New("argument 'router' cannot be nil"))
	}
	if factory == nil {
		panic(errors.New("argument 'factory' cannot be nil"))
	}
	return Dispatcher{name, router, factory}
}

func (di Dispatcher) String() string {
	return fmt.Sprintf("di.Dispatcher<%s>", di.name)
}

// validate methods that will handle requests
func validate(meth reflect.Method) error {
	// PkgPath will be empty for exported names, since a method name will
	// not be unique in a method set if it is spelled the same and is
	// exported. See http://golang.org/ref/spec#Uniqueness_of_identifiers
	//
	// Since 1.7, the Method and NumMethod methods of Type and Value no longer
	// return or count unexported methods. That makes this test redundant.
	if meth.PkgPath != "" {
		return errors.New("not an exported type")
	}

	// acceptable methods should have 3 ins:
	// receiver, http.ResponseWriter, *http.Request
	expectedNumIn := 3
	if numIn := meth.Type.NumIn(); numIn != expectedNumIn {
		return fmt.Errorf("wrong number of arguments: %d, expect %d", numIn, expectedNumIn)
	}

	// There is no need to validate that the receiver implements type
	// Controller as that is how we got here.

	// Using the return of reflect.TypeOf(http.ResponseWriter(nil)) causes
	// Type.Implements to panic with
	// panic: reflect: nil type passed to Type.Implements
	// but dereferncing pointer to the interface doesn't
	expectedRespType := reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	if respType := meth.Type.In(1); !respType.Implements(expectedRespType) {
		return fmt.Errorf("1st argument type %s does not implement %s", respType, expectedRespType)
	}

	expectedReqType := reflect.TypeOf((*http.Request)(nil))
	if reqType := meth.Type.In(2); reqType != expectedReqType {
		return fmt.Errorf("2nd argument of type %s, but expect %s", reqType, expectedReqType)
	}
	return nil
}

// adapt returns an http.Handler that gets run in the course of handling a
// request. The handler receives control from the router.ServeHTTP, creates a
// RequestFactory for the request, uses it to get hold the Controller instance
// by name and dispatches it the appropriate method.
func (di Dispatcher) adapt(ctrlType reflect.Type, as string, meth reflect.Method) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		rcvr := di.factory.With(req).NewController(as)
		if rcvrType := reflect.TypeOf(rcvr); rcvrType != ctrlType {
			panic(fmt.Errorf(
				"%s: for %s, %s NewController(%s) returned %s but expected %s",
				di, req.Method, req.URL.Path, as, rcvrType, ctrlType,
			))
		}
		// no need to lookup reflect.Method as we have a reference to the
		// instance looked up during Register time.
		meth.Func.Call([]reflect.Value{reflect.ValueOf(rcvr), reflect.ValueOf(rw), reflect.ValueOf(req)})
	}
}

func (di Dispatcher) bind(ctrl Controller, as string, method Binding) error {
	ctrlType := reflect.TypeOf(ctrl)
	typeName := reflect.Indirect(reflect.ValueOf(ctrl)).Type().Name()
	ctrlMeth, ok := ctrlType.MethodByName(method.Name)
	if !ok {
		return fmt.Errorf("%s: could not find method '%s' in type '%s'", di, method.Name, typeName)
	}

	if err := validate(ctrlMeth); err != nil {
		return fmt.Errorf("%s: error validating %s.%s: %s", di, typeName, method.Name, err)
	}

	adapter := di.adapt(ctrlType, as, ctrlMeth)
	di.router.Handle(strings.ToUpper(method.Verb), method.Path, adapter)
	return nil
}

// Register registers Bindings returned by Controller. It looks up and validates
// that each method of the Binding is of the appropriate type and arranges for
// requests to be delivered to the appropriate methods. Refer to the
// documentation for Binding.
func (di Dispatcher) Register(ctrl Controller, as string) error {
	if as == "" {
		return fmt.Errorf("%s: argument 'as' cannot be empty", di)
	}
	bindings := ctrl.Bindings()
	if len(bindings) == 0 {
		return fmt.Errorf("%s: type '%s' returns 0 bindings", di, as)
	}
	for _, m := range bindings {
		err := di.bind(ctrl, as, m)
		if err != nil {
			return err
		}
	}
	return nil
}
