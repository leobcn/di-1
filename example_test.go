// This example demonstrates injecting Transport into controller. The Transport
// implementation injected depends on the value of the variable config.test.
package di_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/kkrs/di"
	"github.com/kkrs/di/router"
)

// Transport represents the ability to send a message.
type Transport interface {
	Send(from, to, msg string) error
}

// controller handles requests to send messages. Transport is its sole
// dependency.
type controller struct {
	transport Transport // dependency injected
}

// controller would like requests <GET, /send> to be dispatched to its method
// Send.
func (ct controller) Bindings() []di.Binding {
	return []di.Binding{
		{"GET", "/send", "Send"},
	}
}

// Send implements logic to process the HTTP request to send a message. It
// delegtes the actual task of sending the message to Transport.
func (ct controller) Send(rw http.ResponseWriter, req *http.Request) {
	err := ct.transport.Send(req.FormValue("from"), req.FormValue("to"), req.FormValue("msg"))
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

// exampleTransport is a Transport implementation that prints its arguments to stdout.
// That allows for it to be used in an Example test.
type exampleTransport struct{}

func (m exampleTransport) Send(from, to, msg string) error {
	fmt.Printf("%s %s from %s\n", msg, to, from)
	return nil
}

// gaeTransport is a dummy impl that demonstrates what an appengine version would
// look like.
type gaeTransport struct {
	// ctx context.Context
}

// gaeTransport.Send does nothing so it doesn't import appengine.
func (tr gaeTransport) Send(from, to, msg string) error {
	// mail here refers to the appengine mail package
	//
	// mail.Send(tr.ctx, mail.Message{
	//		Sender: from,
	//		To: []string{to},
	//		Body: msg,
	// })
	return nil
}

// application configuration.
type config struct {
	test bool
}

// appFactory implements ApplicationFactory for this app. It gets created with
// the singletons config and exampleTransport.
type appFactory struct {
	config           config
	exampleTransport exampleTransport
}

// With is called by Dispatcher to inject the request object.
func (fa appFactory) With(req *http.Request) di.RequestFactory {
	return reqFactory{
		af: fa, // pass reference to self so reqFactory has access all singletons
	}
}

type reqFactory struct {
	af appFactory // reference to singletons
}

// newTransport returns a Transport implementation depending on the value of
// config.test. This allows Transport implementations that gets injected to be
// configurable.
func (fa reqFactory) newTransport() Transport {
	if fa.af.config.test {
		return fa.af.exampleTransport // return the test impl
	}
	// return the dummy prod impl. A real implementation would do
	//      return gaeTransport{appengine.NewContext(fa.req)}
	return gaeTransport{}
}

func (fa reqFactory) NewController(name string) di.Controller {
	switch name {
	case "message":
		return controller{fa.newTransport()}
	default:
		panic(fmt.Errorf("do not know how to create '%s'", name))
	}
}

func Example() {
	// populate singletons
	factory := appFactory{
		config{true}, // test environment
		exampleTransport{},
	}

	router := router.New()
	dispatcher := di.New("example", router, factory)
	if err := dispatcher.Register(controller{}, "message"); err != nil {
		panic(err)
	}

	query := url.Values{"from": {"kkrs"}, "to": {"world"}, "msg": {"hello"}}
	req, err := http.NewRequest("GET", "/send?"+query.Encode(), nil)
	if err != nil {
		panic(err)
	}
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, req)

	// NOTE: need a blank line before the Output line or the test won't get
	// executed.
	// Verify that controller works as expected.

	// Output: hello world from kkrs
}
