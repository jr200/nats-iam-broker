package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jr200/nats-iam-broker/internal/logging"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// TestCase defines the available test cases.
type TestCase int

const (
	Sub TestCase = iota
	Pub
	PubSub
	Stream
)

// Required argument counts for each test case
const (
	SubReqArgs    = 1 // Subject
	PubReqArgs    = 2 // Subject and message
	PubSubReqArgs = 2 // Subject and message
	StreamReqArgs = 2 // Stream name and subject
)

// String returns the string representation of a TestCase.
func (tc TestCase) String() string {
	return [...]string{"sub", "pub", "pubsub", "stream"}[tc]
}

// ParseTestCase converts a string to a TestCase.
func ParseTestCase(s string) (TestCase, error) {
	switch s {
	case "sub":
		return Sub, nil
	case "pub":
		return Pub, nil
	case "pubsub":
		return PubSub, nil
	case "stream":
		return Stream, nil
	default:
		return 0, fmt.Errorf("invalid test case: %s", s)
	}
}

// AllTestCases returns all available test cases.
func AllTestCases() []TestCase {
	return []TestCase{Sub, Pub, PubSub, Stream}
}

// ListTestCases returns a comma-separated string of all test cases.
func ListTestCases() string {
	var cases []string
	for _, tc := range AllTestCases() {
		cases = append(cases, tc.String())
	}
	return strings.Join(cases, ", ")
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[client stderr] %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		logLevel    string
		natsURL     string
		credsFile   string
		idpJwt      string
		token       string
		testCaseStr string
		clientSleep int64
	)

	flag.StringVar(&logLevel, "log", "info", "set log-level: disabled, panic, fatal, error, warn, info, debug, trace")
	flag.StringVar(&natsURL, "url", nats.DefaultURL, "NATS URL")
	flag.StringVar(&credsFile, "creds", "", "NATS credentials file")
	flag.StringVar(&idpJwt, "jwt", "", "IdP id_token JWT")
	flag.StringVar(&token, "token", "", "IdP authentication id_token JWT (overrides JWT if both are provided)")
	flag.Int64Var(&clientSleep, "wait", 1, "seconds to wait for client to exit (default=1)")
	flag.StringVar(&testCaseStr, "run-test", "", fmt.Sprintf("nats test to run (%s)", ListTestCases()))

	flag.Parse()
	logging.Setup(logLevel, true)
	defer func() { _ = zap.L().Sync() }()

	if testCaseStr != "" {
		zap.L().Info("running test", zap.String("type", testCaseStr), zap.Strings("args", flag.Args()))
	}

	zap.L().Debug("sending jwt", zap.String("jwt", idpJwt))
	if token != "" {
		zap.L().Debug("using authentication token")
	}

	var natsErr = false
	var nc *nats.Conn

	options := []nats.Option{
		nats.LameDuckModeHandler(func(_ *nats.Conn) {
			zap.L().Info("Incoming Event, LDM. Client has been requested to reconnect")
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			zap.L().Info("nats client reconnected")
		}),
		nats.ConnectHandler(func(_ *nats.Conn) {
			zap.L().Info("connected to nats", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.Name("test-client"),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			zap.L().Error("received nats-error", zap.Error(err))
			natsErr = true
		}),
	}

	if credsFile != "" {
		options = append(options, nats.UserCredentials(credsFile))
	}

	if token != "" {
		options = append(options, nats.Token(token))
	} else if idpJwt != "" {
		options = append(options, nats.Token(idpJwt))
	}

	var connectionErr error
	nc, connectionErr = nats.Connect(natsURL, options...)
	if connectionErr != nil {
		zap.L().Error("failed to connect", zap.Error(connectionErr))
		// If not running a specific test, return the error immediately
		if testCaseStr == "" {
			return fmt.Errorf("failed to connect: %w", connectionErr)
		}
		// Otherwise, nc will be nil, and the test failure will be handled below
	}

	if testCaseStr != "" {
		testFailed := false
		testArgs := flag.Args()
		var testErr error

		if nc == nil { // Check if connection failed earlier
			zap.L().Error("test failed due to connection error", zap.String("test", testCaseStr))
			testFailed = true
		} else {
			// Connection succeeded, proceed with test case
			testErr = runTestCase(nc, testCaseStr, testArgs)
			time.Sleep(time.Duration(clientSleep) * time.Second)

			if testErr != nil {
				zap.L().Error("test failed", zap.String("test", testCaseStr), zap.Error(testErr))
				testFailed = true
			}
			if natsErr { // natsErr is set by the async error handler
				zap.L().Error("test failed due to nats error", zap.String("test", testCaseStr))
				testFailed = true
			}
		}

		if !testFailed {
			zap.L().Info("test successful", zap.String("type", testCaseStr), zap.Strings("args", flag.Args()))
		}
	}

	// Only drain if the connection was successful
	if nc != nil {
		if err := nc.Drain(); err != nil {
			zap.L().Error("error draining NATS connection", zap.Error(err))
		}
	}

	return nil
}

func runTestCase(nc *nats.Conn, testCaseStr string, args []string) error {
	zap.L().Debug("in runTestCase", zap.String("type", testCaseStr), zap.Strings("args", args))

	if testCaseStr == "" {
		zap.L().Debug("no test to run")
		return nil
	}

	testCase, err := ParseTestCase(testCaseStr)
	if err != nil {
		return fmt.Errorf("%v. Valid cases are: %s", err, ListTestCases())
	}

	switch testCase {
	case Sub:
		if len(args) < SubReqArgs {
			return errors.New("sub test requires a subject")
		}
		ns, err := nc.Subscribe(args[0], func(msg *nats.Msg) {
			zap.L().Info("got-msg", zap.ByteString("data", msg.Data))
			_ = msg.Ack()
		})

		if err != nil {
			return err
		}

		if !ns.IsValid() {
			return errors.New("invalid subscription")
		}

		zap.L().Info("subscribed ok", zap.Any("subscription", ns))

	case Pub:
		if len(args) < PubReqArgs {
			return errors.New("pub test requires a subject and a message")
		}
		err := nc.Publish(args[0], []byte(args[1]))
		if err != nil {
			return err
		}

	case PubSub:
		if len(args) < PubSubReqArgs {
			return errors.New("pubsub test requires a subject and a message")
		}
		_, err := nc.Subscribe(args[0], func(msg *nats.Msg) {
			zap.L().Info("got-msg", zap.ByteString("data", msg.Data))
			_ = msg.Ack()
		})
		if err != nil {
			return err
		}

		err = nc.Publish(args[0], []byte(args[1]))
		if err != nil {
			return err
		}

	case Stream:
		if len(args) < StreamReqArgs {
			return errors.New("stream test requires a stream name and a subject")
		}
		streamName := args[0]
		subject := args[1]
		js, err := jetstream.New(nc)
		if err != nil {
			return err
		}

		stream, err := createStream(js, streamName, subject)
		if err != nil {
			return err
		}

		msgCount := 5
		err = publishJetstream(js, subject, msgCount)
		if err != nil {
			return err
		}
		err = subscribeJetstream(stream, streamName, msgCount)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown test-case logic error: %s", testCase)
	}

	return nil
}

func createStream(js jetstream.JetStream, streamName string, subject string) (jetstream.Stream, error) {
	const maxBytes = 10 * 1024
	ctx := context.Background()
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        streamName,
		Description: fmt.Sprintf("test stream %s", streamName),
		Subjects:    []string{subject},
		MaxBytes:    maxBytes,
	})
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func publishJetstream(js jetstream.JetStream, sub string, msgCount int) error {
	for i := 0; i < msgCount; i++ {
		msg := fmt.Sprintf("js-msg %d", i)
		_, err := js.Publish(context.Background(), sub, []byte(msg))
		if err != nil {
			return err
		}
	}

	return nil
}

func subscribeJetstream(js jetstream.Stream, name string, msgCount int) error {
	wg := &sync.WaitGroup{}
	wg.Add(msgCount)

	consumer, err := js.CreateOrUpdateConsumer(context.Background(), jetstream.ConsumerConfig{
		Name:    name,
		Durable: name,
	})
	if err != nil {
		return err
	}
	c, err := consumer.Consume(func(msg jetstream.Msg) {
		zap.L().Info("consumed-msg", zap.String("subject", msg.Subject()), zap.ByteString("data", msg.Data()))
		_ = msg.Ack()
		wg.Done()
	})
	if err != nil {
		return err
	}

	wg.Wait()
	c.Stop()

	return nil
}
