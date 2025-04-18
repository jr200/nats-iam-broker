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

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		fmt.Fprintf(os.Stderr, "[client stderr] %v", err)
		os.Exit(1)
	}
}

func configureLogging(logLevel string, logHumanReadable bool) {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if logHumanReadable {
		log.Logger = log.With().Str("app", "client").Logger().Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
}

func run() error {
	var (
		logLevel    string
		natsURL     string
		credsFile   string
		idpJwt      string
		testCaseStr string
		clientSleep int64
	)

	flag.StringVar(&logLevel, "log", "info", "set log-level: disabled, panic, fatal, error, warn, info, debug, trace")
	flag.StringVar(&natsURL, "url", nats.DefaultURL, "NATS URL")
	flag.StringVar(&credsFile, "creds", "", "NATS credentials file")
	flag.StringVar(&idpJwt, "jwt", "", "IdP id_token JWT")
	flag.Int64Var(&clientSleep, "wait", 1, "seconds to wait for client to exit (default=1)")
	flag.StringVar(&testCaseStr, "run-test", "", fmt.Sprintf("nats test to run (%s)", ListTestCases()))

	flag.Parse()
	configureLogging(logLevel, true)

	log.Info().Msgf("connecting to %s", natsURL)
	log.Trace().Msgf("sending jwt %s", idpJwt)

	var natsErr = false

	nc, err := nats.Connect(
		natsURL,
		nats.UserCredentials(credsFile),
		nats.UserInfo("jwt", idpJwt),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			natsErr = true
			log.Err(err).Msgf("received nats-error")
		}),
	)

	if err != nil {
		log.Err(err).Msg("failed to connect")
		return err
	}

	log.Info().Msgf("successful connection to %s", nc.ConnectedUrl())

	if testCaseStr != "" {
		// Get remaining positional arguments
		testArgs := flag.Args()
		testErr := runTestCase(nc, testCaseStr, testArgs)
		time.Sleep(time.Duration(clientSleep) * time.Second)

		switch {
		case testErr != nil:
			log.Err(testErr).Msg("test failed. err from test-client.")
		case natsErr:
			log.Error().Msg("test failed. err from nats-server")
		default:
			log.Info().Msg("test successful")
		}
	}

	if err := nc.Drain(); err != nil {
		log.Err(err).Msg("error draining NATS connection")
	}

	return nil
}

func runTestCase(nc *nats.Conn, testCaseStr string, args []string) error {
	log.Info().Msgf("test-case type: %s, args: %v", testCaseStr, args)

	if testCaseStr == "" {
		log.Trace().Msg("no test to run")
		return nil
	}

	testCase, err := ParseTestCase(testCaseStr)
	if err != nil {
		return fmt.Errorf("%v. Valid cases are: %s", err, ListTestCases())
	}

	log.Info().Msgf("running test-case: %s", testCase)

	switch testCase {
	case Sub:
		if len(args) < SubReqArgs {
			return errors.New("sub test requires a subject")
		}
		ns, err := nc.Subscribe(args[0], func(msg *nats.Msg) {
			log.Info().Msgf("got-msg: %s", msg.Data)
			_ = msg.Ack()
		})

		if err != nil {
			return err
		}

		if !ns.IsValid() {
			return errors.New("invalid subscription")
		}

		log.Info().Msgf("subscribed ok: %v", ns)

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
			log.Info().Msgf("got-msg: %s", msg.Data)
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
		log.Info().Msgf("consumed-msg[%s]: %s", msg.Subject(), msg.Data())
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
