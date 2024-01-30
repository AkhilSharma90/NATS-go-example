package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	"github.com/ThreeDotsLabs/watermill/message"
	nc "github.com/nats-io/nats.go"
)

// push-based consumer example
func main() {
	marshaler := &nats.NATSMarshaler{}
	logger := watermill.NewStdLogger(false, false)
	options := []nc.Option{
		nc.RetryOnFailedConnect(true),
		nc.Timeout(30 * time.Second),
		nc.ReconnectWait(1 * time.Second),
	}

	// jsSubOptions are JetStream-specific configurations
	jsSubOptions := []nc.SubOpt{
		// MaxAckPending sets the number of outstanding acks that are allowed before message delivery is halted
		// if it is too large, the subscriber will have not enough time processing messages
		// before NATS timeout. In this case, NATS will send the same batch of messages
		// to another subscriber in the same queue group. Thus, messages may be processed twice
		nc.MaxAckPending(2048),

		// MaxDeliver sets the number of redeliveries for a message
		// Applies to any message that is re-sent due to a negative ack, or no ack sent by the client
		nc.MaxDeliver(15),
		nc.AckExplicit(),

		// LimitsPolicy (default) means that messages are retained until any given limit is reached
		// This could be one of MaxMsgs, MaxBytes, or MaxAge.

		// Discard Policy can be either Old (default) or New. It affects how MaxMessages and MaxBytes operate.
		// If a limit is reached and the policy is Old, the oldest message is removed.
		// If the policy is New, new messages are refused if it would put the stream over the limit.

		// InactiveThreshold indicates how long the server should keep a consumer
		// after detecting a lack of activity. In NATS Server 2.8.4 and earlier, this
		// option only applies to ephemeral consumers. In NATS Server 2.9.0 and later,
		// this option applies to both ephemeral and durable consumers, allowing durable
		// consumers to also be deleted automatically after the inactivity threshold has passed
		// (By default, durables will remain even when there are periods of inactivity unless InactiveThreshold is set explicitly)
		nc.InactiveThreshold(300 * time.Second),
	}

	// if JetStreamConfig.Disabled is set to true, then core NATS subscription is used
	// - If QueueGroup is not empty, then at-most-once queue group pattern will be used
	// - If QueueGroup is empty, then at-most-once fan-out push pattern will be used
	//   SubscribersCount should be set to 1 to avoid duplication
	jsConfig := nats.JetStreamConfig{
		Disabled:         false,
		AutoProvision:    false,
		SubscribeOptions: jsSubOptions,
		TrackMsgId:       false,
		// use msg.Ack(), which tells the NTS server that the message was successfully processed and it can move on to the next message
		AckAsync:      true,
		DurablePrefix: "my-durable",
	}

	// the following comments are JetStream specific, ie. discussion on durability (JetStreamConfig.Disabled = false)
	subscriber1, err := nats.NewSubscriber(
		nats.SubscriberConfig{
			URL: os.Getenv("NATS_URL"),
			// A durable queue group (non-empty DurablePrefix) allows you to have all members leave
			// but still maintain state. When a member re-joins, it starts at the last position in that group.
			// If using empty DurablePrefix or no binding options being specified, the queue name will be used as a durable name

			// When QueueGroup is empty, subscribe without QueueGroup (default subscribe, fan-out push pattern) will be used
			// - If using non-empty DurablePrefix with default subscribe, the library will attempt to lookup a JetStream
			//   consumer with this name, and if found, will bind to it and not attempt to delete it.
			//   However, if not found, the library will send a request to create such durable JetStream consumer.
			//   Now JetStream will persist the position of the durable consumer over the stream
			//   Note that DurablePrefix should be unique for each subscriber here to avoid duplication
			// - If using empty DurablePrefix with default subscribe, the library will send a request to the server
			//   to create an ephemeral JetStream consumer, which will be deleted after an Unsubscribe() or Drain()
			//   or after InactiveThreshold (defaults to 5 seconds) is reached when not actively consuming messages
			//   Ephemeral consumers are meant to be used by a single instance of an application (e.g. to get its own replay of the messages in the stream)
			// In both case, SubscribersCount should be set to 1 to avoid duplication
			QueueGroupPrefix: "example",
			SubscribersCount: 4, // how many goroutines should consume messages
			CloseTimeout:     time.Minute,
			// How long subscriber should wait for Ack/Nack. When no Ack/Nack was received, message will be redelivered.
			AckWaitTimeout: time.Second * 30,
			NatsOptions:    options,
			Unmarshaler:    marshaler,
			JetStream:      jsConfig,
		},
		logger,
	)
	if err != nil {
		panic(err)
	}

	subscriber2, err := nats.NewSubscriber(
		nats.SubscriberConfig{
			URL:              os.Getenv("NATS_URL"),
			QueueGroupPrefix: "example",
			SubscribersCount: 4,
			CloseTimeout:     time.Minute,
			AckWaitTimeout:   time.Second * 30,
			NatsOptions:      options,
			Unmarshaler:      marshaler,
			JetStream:        jsConfig,
		},
		logger,
	)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal - closing subscriber")
		subscriber1.Close()
		subscriber2.Close()
		os.Exit(0)
	}()

	messages1, err := subscriber1.Subscribe(context.Background(), "example_topic.>")
	if err != nil {
		panic(err)
	}
	go processJS(messages1, "subscriber1")

	messages2, err := subscriber2.Subscribe(context.Background(), "example_topic.>")
	if err != nil {
		panic(err)
	}
	go processJS(messages2, "subscriber2")

	publisher, err := nats.NewPublisher(
		nats.PublisherConfig{
			URL:         os.Getenv("NATS_URL"),
			NatsOptions: options,
			Marshaler:   marshaler,
			JetStream: nats.JetStreamConfig{
				Disabled:      false,
				AutoProvision: false,
				ConnectOptions: []nc.JSOpt{
					// the maximum outstanding async publishes that can be inflight at one time
					nc.PublishAsyncMaxPending(16384),
				},
				PublishOptions: nil,
				// enable idempotent message writes by ignoring duplicate messages as indicated by the Nats-Msg-Id header
				TrackMsgId: false,
			},
		},
		logger,
	)
	if err != nil {
		panic(err)
	}

	i := 0
	var id string
	for {
		id = strconv.Itoa(i)
		msgA := message.NewMessage(id, []byte("hello from a"))
		msgB := message.NewMessage(id, []byte("hello from b"))
		msgATest := message.NewMessage(id, []byte("hello from a.test"))
		msgBTest := message.NewMessage(id, []byte("hello from b.test"))

		if err := publisher.Publish("example_topic.a", msgA); err != nil {
			panic(err)
		}
		if err := publisher.Publish("example_topic.b", msgB); err != nil {
			panic(err)
		}

		if err := publisher.Publish("example_topic.a.test", msgATest); err != nil {
			panic(err)
		}
		if err := publisher.Publish("example_topic.b.test", msgBTest); err != nil {
			panic(err)
		}

		time.Sleep(time.Second)
		i++
	}
}

func processJS(messages <-chan *message.Message, from string) {
	for msg := range messages {
		log.Printf("[%s] received message: %s, payload: %s", from, msg.UUID, string(msg.Payload))

		// we need to Acknowledge that we received and processed the message,
		// otherwise, it will be resent over and over again.
		msg.Ack()
	}
}
