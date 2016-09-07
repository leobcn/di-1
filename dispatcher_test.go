package di_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/kkrs/di"
	"github.com/kkrs/di/router"
)

func mustResponseReq(method, path string, body io.ReadCloser) (*httptest.ResponseRecorder, *http.Request) {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		panic(err)
	}
	return httptest.NewRecorder(), req
}

type panicController struct{}

func (panicController) Bindings() []di.Binding {
	return []di.Binding{{"GET", "/", "Handle"}}
}

func (panicController) Handle(http.ResponseWriter, *http.Request) {}

type panicFactory struct{}

func (fa panicFactory) With(*http.Request) di.RequestFactory {
	return fa // panicFactory implements RequestFactory as well
}

func (panicFactory) NewController(string) di.Controller {
	return dummyController{} // expected to return PanicController
}

func TestServePanic(t *testing.T) {
	t.Logf("Dispatcher panics because RequestFactory returns a different concrete type from the one registered by the controller")
	router := router.New()
	dispatcher := di.New("panic", router, panicFactory{})

	// registered with panicController, however panicFactory returns dummyController
	if err := dispatcher.Register(panicController{}, "panic"); err != nil {
		panic(err)
	}

	rw, req := mustResponseReq("GET", "/", nil)
	defer func() {
		if recovered := recover(); recovered != nil {
			got, ok := recovered.(error)
			if !ok {
				t.Fatalf("expected recovered value to be of type error, got %#v", recovered)
			}
			expected := fmt.Errorf("di.Dispatcher<panic>: for GET, / NewController(panic) returned di_test.dummyController but expected di_test.panicController")
			if !reflect.DeepEqual(got, expected) {
				t.Logf("expected: %#v", expected)
				t.Fatalf("got:      %#v", got)
			}
		}
	}()
	router.ServeHTTP(rw, req)
}

type dummyController struct{}

func (dummyController) Bindings() []di.Binding {
	return nil
}

func (dummyController) Handle(http.ResponseWriter, *http.Request) {}

type dummyRequestFactory struct{}

func (dummyRequestFactory) NewController(string) di.Controller {
	return dummyController{}
}

type dummyFactory struct{}

func (dummyFactory) With(*http.Request) di.RequestFactory {
	return dummyRequestFactory{}
}

type missingMethod struct{}

func (missingMethod) Bindings() []di.Binding {
	return []di.Binding{
		{"GET", "/missing", "missing"},
	}
}

type unexported struct{}

func (unexported) Bindings() []di.Binding {
	return []di.Binding{
		{"GET", "/unexported", "method"},
	}
}

// method signature is wrong as well
func (unexported) method() {}

type wrongNumber struct{}

func (wrongNumber) Bindings() []di.Binding {
	return []di.Binding{
		{"GET", "/wrongNumber", "Args"},
	}
}
func (wrongNumber) Args() {}

type wrongFirst struct{}

func (wrongFirst) Bindings() []di.Binding {
	return []di.Binding{
		{"GET", "/wrongFirst", "Arg"},
	}
}

func (wrongFirst) Arg(*http.Request, *http.Request) {}

type wrongSecond struct{}

func (wrongSecond) Bindings() []di.Binding {
	return []di.Binding{
		{"GET", "/wrongnSecond", "Arg"},
	}
}

func (wrongSecond) Arg(http.ResponseWriter, http.ResponseWriter) {}

func testValidateCase(
	t *testing.T,
	scenario string,
	dis di.Dispatcher,
	ctrl di.Controller,
	as string,
	expected string,
) {
	t.Log(scenario)
	if got := dis.Register(ctrl, as); got.Error() != expected {
		t.Errorf("expected: %s", expected)
		t.Fatalf("got:      %s", got.Error())
	}
	t.Log("")
}

func TestValidationErrors(t *testing.T) {
	dis := di.New("ValidateErrors", router.New(), dummyFactory{})

	prefix := "di.Dispatcher<ValidateErrors>"
	testValidateCase(
		t, "Dispatcher.Register errors because argument 'as' is empty",
		dis, dummyController{}, "", fmt.Sprintf("%s: argument 'as' cannot be empty", prefix),
	)

	testValidateCase(
		t, "Dispatcher.Register errors because Controller type passed returns 0 bindings",
		dis, dummyController{}, "dummy", fmt.Sprintf("%s: type 'dummy' returns 0 bindings", prefix),
	)

	testValidateCase(
		t, "Dispatcher.Register errors because Controller method exported could not be found",
		dis, missingMethod{}, "missing",
		fmt.Sprintf("%s: could not find method 'missing' in type 'missingMethod'", prefix),
	)

	var (
		maj      = 0
		err      error
		expected string
	)
	// runtime.Version returns string of the form "go1.7", extract 7
	if ver := runtime.Version(); len(ver) > 4 {
		maj, err = strconv.Atoi(string(ver[4]))
		if err != nil {
			panic(err)
		}
	}

	// go1.7 does not return unexported method for Type.
	switch {
	case maj < 7:
		expected = fmt.Sprintf("%s: error validating unexported.method: not an exported type", dis)
	default:
		expected = fmt.Sprintf("%s: could not find method 'method' in type 'unexported'", dis)
	}

	testValidateCase(
		t, "Dispatcher.Register errors because Controller method is not an exported type",
		dis, unexported{}, "unexported",
		expected,
	)

	testValidateCase(
		t, "Dispatcher.Register errors because Controller method has the wrong number of arguments",
		dis, wrongNumber{}, "wrongNumber",
		fmt.Sprintf(
			"%s: error validating wrongNumber.Args: wrong number of arguments: 1, expect 3", dis,
		),
	)

	testValidateCase(
		t, "Dispatcher.Register errors because Controller handler's 1st arg is of the wrong type",
		dis, wrongFirst{}, "wrongFirst",
		fmt.Sprintf(
			"%s: error validating wrongFirst.Arg: 1st argument type *http.Request does not implement http.ResponseWriter",
			dis,
		),
	)

	testValidateCase(
		t, "Dispatcher.Register errors because Controller handler's 2nd arg is of the wrong type",
		dis, wrongSecond{}, "wrongSecond",
		fmt.Sprintf(
			"%s: error validating wrongSecond.Arg: 2nd argument of type http.ResponseWriter, but expect *http.Request",
			dis,
		),
	)
}

func testNewPanicCase(t *testing.T, name string, rtr di.Router, fac di.ApplicationFactory, expected string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			got, ok := recovered.(error)
			if !ok {
				t.Fatalf("expected type error, got %#v", recovered)
			}
			if got.Error() != expected {
				t.Errorf("expected: %s", expected)
				t.Fatalf("got:      %s", got.Error())
			}
		}
	}()
	di.New(name, rtr, fac)
}

func TestNewPanics(t *testing.T) {
	testNewPanicCase(t, "", nil, nil, "argument 'name' cannot be empty")
	testNewPanicCase(t, "notempty", nil, nil, "argument 'router' cannot be nil")
	testNewPanicCase(t, "notempty", router.New(), nil, "argument 'factory' cannot be nil")
}
