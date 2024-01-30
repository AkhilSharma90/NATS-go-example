# NATS PubSub with GO Example

Note from akhil- 
I got interested in NATS after watching Stephen Grider's Javascript Microservices tutorial long back. Wanted to see how NATS can work with GO, couldn't find other examples on the internet.

The easiest way I found was to use Watermill for NATS.
P.S - 

1. It might be tempting to try our Kafka with watermill - however, I recommend against it, there are much better ways to use Kafka with GO and you will find them on my github.

2. You don't have to worry about creating an env file for NATS, the program is reading from docker (I've done the work for you)

3. You must have set your GOPATH before you run this example.

[Watermill](https://watermill.io/). The application runs in a loop, consuming events from a NATS JetStream.

This is an example for NATS push-based consumers. 

There's a docker-compose file included, so you can run the example and see it in action.

## Files

- [main.go](main.go) - example source code
- [docker-compose.yml](docker-compose.yml) - local environment Docker Compose configuration
- [go.mod](go.mod) - Go modules dependencies, you can find more information at [Go wiki](https://github.com/golang/go/wiki/Modules)
- [go.sum](go.sum) - Go modules checksums

## Requirements

To run this example you will need Docker and docker-compose installed. See the [installation guide](https://docs.docker.com/compose/install/).

## Result
`subscriber1` and `subscriber2` are in the same queue group `example`, and they both subscribe to `example_topic.>`. In each round, `publisher` publishes four messages to `example_topic.a`, `example_topic.b`, `example_topic.a.test`, and `example_topic.b.test` respectively. We can see that both `subscriber1` and `subscriber2` can receive messages from all four subjects, and each message is processed only once by either `subscriber1` or `subscriber2` since they are in the same queue group.
```
> docker-compose up

2023/09/20 12:03:07 [subscriber1] received message: 0, payload: hello from a
2023/09/20 12:03:07 [subscriber1] received message: 0, payload: hello from b
2023/09/20 12:03:07 [subscriber2] received message: 0, payload: hello from a.test
2023/09/20 12:03:07 [subscriber2] received message: 0, payload: hello from b.test
2023/09/20 12:03:08 [subscriber2] received message: 1, payload: hello from a
2023/09/20 12:03:08 [subscriber2] received message: 1, payload: hello from b
2023/09/20 12:03:08 [subscriber2] received message: 1, payload: hello from a.test
2023/09/20 12:03:08 [subscriber1] received message: 1, payload: hello from b.test
2023/09/20 12:03:09 [subscriber1] received message: 2, payload: hello from a
2023/09/20 12:03:09 [subscriber1] received message: 2, payload: hello from b
2023/09/20 12:03:09 [subscriber1] received message: 2, payload: hello from a.test
2023/09/20 12:03:09 [subscriber2] received message: 2, payload: hello from b.test
2023/09/20 12:03:10 [subscriber2] received message: 3, payload: hello from a
2023/09/20 12:03:10 [subscriber1] received message: 3, payload: hello from b
2023/09/20 12:03:10 [subscriber1] received message: 3, payload: hello from a.test
2023/09/20 12:03:10 [subscriber1] received message: 3, payload: hello from b.test
...
```