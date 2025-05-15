package kafka

import (
    "context"
    "time"
    "log"

    "github.com/segmentio/kafka-go"
)

// Writer is the global Kafka writer instance.
var Writer *kafka.Writer

// Init sets up the global Writer with the given broker addresses and topic.
// Call this once at application startup before sending messages.
func Init(brokers []string, topic string) {
    Writer = kafka.NewWriter(kafka.WriterConfig{
        Brokers: brokers,
        Topic:   topic,
    })
}

// Send publishes one message synchronously, blocking until the broker acknowledges
// or an error occurs. Returns an error if the write fails.
func Send(ctx context.Context, key, value []byte) error {
    return Writer.WriteMessages(ctx,
        kafka.Message{Key: key, Value: value},
    )
}

// SendAsync publishes one message in the background using a detached context,
// so it is not canceled when the caller's context is done. It logs any error.
func SendAsync(key, value []byte) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    go func() {
        if err := Writer.WriteMessages(ctx,
            kafka.Message{Key: key, Value: value},
        ); err != nil {
            log.Printf("⚠️ kafka send error: %v", err)
        }
    }()
}
