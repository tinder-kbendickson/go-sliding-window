package swp

import (
	"fmt"
	"time"

	"github.com/nats-io/gnatsd/server"
	gnatsd "github.com/nats-io/gnatsd/test"
	"github.com/nats-io/nats"
	"io/ioutil"
	"net"

	"os"
)

// not meant to be run on its own, but shows an example
// of all the setup and teardown in a test.
func exampleSetup_test() {
	origdir, tempdir := makeAndMoveToTempDir() // cd to tempdir
	p("origdir = '%s'", origdir)
	p("tempdir = '%s'", tempdir)
	defer tempDirCleanup(origdir, tempdir)

	host := "127.0.0.1"
	port := getAvailPort()
	gnats := startGnatsd(host, port)
	defer func() {
		p("calling gnats.Shutdown()")
		gnats.Shutdown() // when done
	}()

	subC := NewNatsClientConfig(host, port, "B-subscriber", "toB", true, true)
	sub := NewNatsClient(subC)
	err := sub.Start()
	panicOn(err)
	defer sub.Close()

	pubC := NewNatsClientConfig(host, port, "A-publisher", "toA", true, true)
	pub := NewNatsClient(pubC)
	err = pub.Start()
	panicOn(err)
	defer pub.Close()

	p("sub = %#v", sub)
	p("pub = %#v", pub)
}

func makeAndMoveToTempDir() (origdir string, tmpdir string) {

	// make new temp dir that will have no ".goqclusterid files in it
	var err error
	origdir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	tmpdir, err = ioutil.TempDir(origdir, "temp-profiler-testdir")
	if err != nil {
		panic(err)
	}
	err = os.Chdir(tmpdir)
	if err != nil {
		panic(err)
	}

	return origdir, tmpdir
}

func tempDirCleanup(origdir string, tmpdir string) {
	// cleanup
	os.Chdir(origdir)
	err := os.RemoveAll(tmpdir)
	if err != nil {
		panic(err)
	}
	q("\n TempDirCleanup of '%s' done.\n", tmpdir)
}

// getAvailPort asks the OS for an unused port.
// There's a race here, where the port could be grabbed by someone else
// before the caller gets to Listen on it, but in practice such races
// are rare. Uses net.Listen("tcp", ":0") to determine a free port, then
// releases it back to the OS with Listener.Close().
func getAvailPort() int {
	l, _ := net.Listen("tcp", ":0")
	r := l.Addr()
	l.Close()
	return r.(*net.TCPAddr).Port
}

func startGnatsd(host string, port int) *server.Server {
	//serverList := fmt.Sprintf("nats://%v:%v", host, port)

	// start yourself an embedded gnatsd server
	opts := server.Options{
		Host:  host,
		Port:  port,
		Trace: true,
		Debug: true,
	}
	gnats := gnatsd.RunServer(&opts)
	//gnats.SetLogger(&Logger{}, true, true)

	//logger := log.New(os.Stderr, "gnatsd: ", log.LUTC|log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	addr := fmt.Sprintf("%v:%v", host, port)
	if !portIsBound(addr) {
		panic("port not bound " + addr)
	}
	return gnats
}

func portIsBound(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// NatsClient wraps a nats.Conn, a nats.Subscription,
// and a message arrival channel into an easy to use
// and easy to configure (even with TLS) structure.
type NatsClient struct {
	Nc           *nats.Conn
	Scrip        *nats.Subscription
	MsgArrivalCh chan *nats.Msg
	Cfg          *NatsClientConfig
	Subject      string
}

// NewNatsClient creates a new NatsClient.
func NewNatsClient(cfg *NatsClientConfig) *NatsClient {
	c := &NatsClient{
		MsgArrivalCh: make(chan *nats.Msg),
		Cfg:          cfg,
		Subject:      cfg.Subject,
	}
	return c
}

// Start connects to the gnatsd server.
func (s *NatsClient) Start() error {
	//	s.Cfg.AsyncErrPanics = true
	s.Cfg.Init()

	nc, err := nats.Connect(s.Cfg.ServerList, s.Cfg.Opts...)
	panicOn(err)
	p("client connection succeeded.")
	s.Nc = nc

	return nil
}

// MakeSub creates a nats subscription on subject with the
// hand as the callback handler.
func (s *NatsClient) MakeSub(subject string, hand nats.MsgHandler) error {
	var err error
	s.Scrip, err = s.Nc.Subscribe(s.Subject, hand)
	panicOn(err)
	return err
}

// Close unsubscribes from the nats subscription and closes
// the nats.Conn connection.
func (s *NatsClient) Close() {
	if s.Scrip != nil {
		err := s.Scrip.Unsubscribe()
		panicOn(err)
	}
	if s.Nc != nil {
		s.Nc.Close()
	}
	p("NatsClient unsubscribe and close done")
}

type asyncErr struct {
	conn *nats.Conn
	sub  *nats.Subscription
	err  error
}

// NewNatsClientConfig creates a new config struct
// to provide to NewNatsClient.
func NewNatsClientConfig(
	host string,
	port int,
	myname string,
	subject string,
	skipTLS bool,
	asyncErrCrash bool) *NatsClientConfig {

	cfg := &NatsClientConfig{
		Host:           host,
		Port:           port,
		NatsNodeName:   myname,
		Subject:        subject,
		SkipTLS:        skipTLS,
		AsyncErrPanics: asyncErrCrash,
		ServerList:     fmt.Sprintf("nats://%v:%v", host, port),
	}
	return cfg
}

// NatsClientConfig provides the nats configuration
// for the NatsClient.
type NatsClientConfig struct {
	// ====================
	// user supplied
	// ====================
	Host string
	Port int

	NatsNodeName string
	Subject      string

	SkipTLS bool

	// helpful for test code to auto-crash on error
	AsyncErrPanics bool

	// ====================
	// Init() fills in:
	// ====================
	ServerList string

	NatsAsyncErrCh   chan asyncErr
	NatsConnClosedCh chan *nats.Conn
	NatsConnDisconCh chan *nats.Conn
	NatsConnReconCh  chan *nats.Conn

	Opts  []nats.Option
	Certs certConfig
}

// Init initializes the nats options and
// loads the TLS certificates, if any.
func (cfg *NatsClientConfig) Init() {

	if !cfg.SkipTLS && !cfg.Certs.skipTLS {
		err := cfg.Certs.certLoad()
		if err != nil {
			panic(err)
		}
	}

	o := []nats.Option{}
	o = append(o, nats.MaxReconnects(-1)) // -1 => keep trying forever
	o = append(o, nats.ReconnectWait(2*time.Second))
	o = append(o, nats.Name(cfg.NatsNodeName))

	o = append(o, nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, e error) {
		if cfg.AsyncErrPanics {
			fmt.Printf("\n  got an asyn err, here is the"+
				" status of nats queues: '%#v'\n",
				ReportOnSubscription(s))
			panic(e)
		}
		cfg.NatsAsyncErrCh <- asyncErr{conn: c, sub: s, err: e}
	}))
	o = append(o, nats.DisconnectHandler(func(conn *nats.Conn) {
		cfg.NatsConnDisconCh <- conn
	}))
	o = append(o, nats.ReconnectHandler(func(conn *nats.Conn) {
		cfg.NatsConnReconCh <- conn
	}))
	o = append(o, nats.ClosedHandler(func(conn *nats.Conn) {
		cfg.NatsConnClosedCh <- conn
	}))

	if !cfg.SkipTLS && !cfg.Certs.skipTLS {
		o = append(o, nats.Secure(cfg.Certs.tlsConfig))
		o = append(o, cfg.Certs.rootCA)
	}

	cfg.Opts = o
}
