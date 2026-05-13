package collectionjob

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaWriter struct {
	writer *kafka.Writer
}

func NewKafkaWriter(brokers []string) *KafkaWriter {
	return &KafkaWriter{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		},
	}
}

func (w *KafkaWriter) Write(ctx context.Context, event Event) error {
	return w.writer.WriteMessages(ctx, kafka.Message{
		Topic: event.Topic,
		Key:   event.Key,
		Value: event.Value,
		Time:  time.Now().UTC(),
	})
}

func (w *KafkaWriter) Close() error {
	return w.writer.Close()
}

type KafkaReader struct {
	reader *kafka.Reader
}

func NewKafkaReader(brokers []string, groupID string, topic string) *KafkaReader {
	return &KafkaReader{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			GroupID:        groupID,
			Topic:          topic,
			MinBytes:       1,
			MaxBytes:       10 << 20,
			CommitInterval: 0,
		}),
	}
}

func (r *KafkaReader) Read(ctx context.Context) (Message, error) {
	msg, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return Message{}, err
	}

	return Message{
		Topic:    msg.Topic,
		Key:      msg.Key,
		Value:    msg.Value,
		metadata: msg,
	}, nil
}

func (r *KafkaReader) Commit(ctx context.Context, msg Message) error {
	if kafkaMessage, ok := msg.metadata.(kafka.Message); ok {
		return r.reader.CommitMessages(ctx, kafkaMessage)
	}

	return r.reader.CommitMessages(ctx, kafka.Message{
		Topic: msg.Topic,
		Key:   msg.Key,
		Value: msg.Value,
	})
}

func (r *KafkaReader) Close() error {
	return r.reader.Close()
}
